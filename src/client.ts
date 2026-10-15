import path from 'node:path';
import { fileURLToPath } from 'node:url';
import grpc from '@grpc/grpc-js';
import protoLoader from '@grpc/proto-loader';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const PROTO_PATH = path.resolve(__dirname, '../proto/learning_service.proto');

const packageDefinition = protoLoader.loadSync(PROTO_PATH, {
  keepCase: true,
  longs: String,
  enums: String,
  defaults: true,
  oneofs: true
});

const protoDescriptor = grpc.loadPackageDefinition(packageDefinition) as any;
const client = new protoDescriptor.learning.v1.LearningService(
  'localhost:50051',
  grpc.credentials.createInsecure()
);

async function main() {
  console.log('[learn-grpc-client] === 1. Testing Unary RPC: GetStage ===');
  await new Promise<void>((resolve, reject) => {
    client.GetStage({ stage_number: 1 }, (err: any, response: any) => {
      if (err) return reject(err);
      console.log('[learn-grpc-client] Received Stage 1:', response.title);
      resolve();
    });
  });

  console.log('\n[learn-grpc-client] === 2. Testing Server Streaming RPC: StreamRoadmap ===');
  await new Promise<void>((resolve, reject) => {
    const stream = client.StreamRoadmap({ category: 'all' });
    stream.on('data', (stage: any) => {
      console.log(`[learn-grpc-client] Streamed Stage ${stage.stage_number}: ${stage.title}`);
    });
    stream.on('end', () => resolve());
    stream.on('error', (err: any) => reject(err));
  });

  console.log('\n[learn-grpc-client] === 3. Testing Client Streaming RPC: RecordMetrics ===');
  await new Promise<void>((resolve, reject) => {
    const stream = client.RecordMetrics((err: any, summary: any) => {
      if (err) return reject(err);
      console.log('[learn-grpc-client] Metrics Summary Result:', summary);
      resolve();
    });

    // Write metric records into stream
    for (let i = 1; i <= 5; i++) {
      stream.write({
        metric_name: 'cpu_usage',
        value: 20.0 + i * 5,
        timestamp: Date.now()
      });
    }
    stream.end();
  });

  console.log('\n[learn-grpc-client] === 4. Testing Bi-directional Streaming RPC: LiveExchange ===');
  await new Promise<void>((resolve, reject) => {
    const bidiStream = client.LiveExchange();

    bidiStream.on('data', (response: any) => {
      console.log(`[learn-grpc-client] Server Response: ${response.message}`);
    });

    bidiStream.on('end', () => resolve());
    bidiStream.on('error', (err: any) => reject(err));

    bidiStream.write({ sender: 'Client-1', message: 'Hello gRPC HTTP/2 Stream!', timestamp: Date.now() });
    bidiStream.write({ sender: 'Client-1', message: 'Second duplex packet', timestamp: Date.now() });
    bidiStream.end();
  });

  console.log('\n[learn-grpc-client] All 4 RPC interaction patterns tested successfully!');
}

main().catch(console.error);
