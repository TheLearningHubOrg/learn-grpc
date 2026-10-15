import { describe, it, before, after } from 'node:test';
import assert from 'node:assert';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import grpc from '@grpc/grpc-js';
import protoLoader from '@grpc/proto-loader';
import { createGrpcServer } from '../src/server.js';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const PROTO_PATH = path.resolve(__dirname, '../proto/learning_service.proto');

describe('gRPC Server & 4 RPC Patterns', () => {
  let server: grpc.Server;
  let client: any;
  let boundPort: number;

  before(async () => {
    server = createGrpcServer();
    await new Promise<void>((resolve, reject) => {
      server.bindAsync('127.0.0.1:0', grpc.ServerCredentials.createInsecure(), (err, port) => {
        if (err) return reject(err);
        boundPort = port;
        resolve();
      });
    });

    const packageDef = protoLoader.loadSync(PROTO_PATH, { keepCase: true, longs: String, defaults: true });
    const proto = (grpc.loadPackageDefinition(packageDef) as any).learning.v1;
    client = new proto.LearningService(`127.0.0.1:${boundPort}`, grpc.credentials.createInsecure());
  });

  after(() => {
    client?.close();
    server?.forceShutdown();
  });

  it('Unary RPC: GetStage should return Stage 1', async () => {
    const res = await new Promise<any>((resolve, reject) => {
      client.GetStage({ stage_number: 1 }, (err: any, response: any) => {
        if (err) return reject(err);
        resolve(response);
      });
    });

    assert.strictEqual(res.stage_number, 1);
    assert.strictEqual(res.title, 'Core Architecture, Protocol Buffers & HTTP/2 Transport');
  });

  it('Unary RPC: GetStage should return NOT_FOUND for unknown stage', async () => {
    await assert.rejects(
      async () => {
        await new Promise((resolve, reject) => {
          client.GetStage({ stage_number: 999 }, (err: any, response: any) => {
            if (err) return reject(err);
            resolve(response);
          });
        });
      },
      (err: any) => {
        assert.strictEqual(err.code, grpc.status.NOT_FOUND);
        return true;
      }
    );
  });

  it('Server Streaming RPC: StreamRoadmap should stream all 7 stages', async () => {
    const stages: any[] = [];
    await new Promise<void>((resolve, reject) => {
      const stream = client.StreamRoadmap({ category: 'all' });
      stream.on('data', (data: any) => stages.push(data));
      stream.on('end', () => resolve());
      stream.on('error', reject);
    });

    assert.strictEqual(stages.length, 7);
    assert.strictEqual(stages[0].stage_number, 1);
    assert.strictEqual(stages[6].stage_number, 7);
  });

  it('Client Streaming RPC: RecordMetrics should aggregate correctly', async () => {
    const summary = await new Promise<any>((resolve, reject) => {
      const stream = client.RecordMetrics((err: any, res: any) => {
        if (err) return reject(err);
        resolve(res);
      });

      stream.write({ metric_name: 'latency', value: 10, timestamp: 1 });
      stream.write({ metric_name: 'latency', value: 30, timestamp: 2 });
      stream.end();
    });

    assert.strictEqual(summary.total_records, 2);
    assert.strictEqual(summary.average_value, 20);
    assert.strictEqual(summary.max_value, 30);
    assert.strictEqual(summary.min_value, 10);
  });
});
