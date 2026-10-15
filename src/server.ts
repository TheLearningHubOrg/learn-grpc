import path from 'node:path';
import { fileURLToPath } from 'node:url';
import grpc from '@grpc/grpc-js';
import protoLoader from '@grpc/proto-loader';
import { STAGES } from './data.js';

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
const learningPackage = protoDescriptor.learning.v1;

export function createGrpcServer(): grpc.Server {
  const server = new grpc.Server();

  server.addService(learningPackage.LearningService.service, {
    // 1. Unary RPC
    GetStage: (call: any, callback: any) => {
      const stageNumber = call.request.stage_number;
      const found = STAGES.find((s) => s.stage_number === stageNumber);

      if (!found) {
        return callback({
          code: grpc.status.NOT_FOUND,
          details: `Stage ${stageNumber} not found in roadmap`
        });
      }

      callback(null, found);
    },

    // 2. Server Streaming RPC
    StreamRoadmap: (call: any) => {
      for (const stage of STAGES) {
        call.write(stage);
      }
      call.end();
    },

    // 3. Client Streaming RPC
    RecordMetrics: (call: any, callback: any) => {
      let count = 0;
      let sum = 0;
      let min = Infinity;
      let max = -Infinity;

      call.on('data', (record: any) => {
        count++;
        sum += record.value;
        if (record.value < min) min = record.value;
        if (record.value > max) max = record.value;
      });

      call.on('end', () => {
        callback(null, {
          total_records: count,
          average_value: count > 0 ? sum / count : 0,
          max_value: count > 0 ? max : 0,
          min_value: count > 0 ? min : 0
        });
      });
    },

    // 4. Bi-directional Streaming RPC
    LiveExchange: (call: any) => {
      call.on('data', (chatMessage: any) => {
        // Echo back with server acknowledgement
        call.write({
          sender: 'Server-Agent',
          message: `ACK [${chatMessage.sender}]: ${chatMessage.message}`,
          timestamp: Date.now()
        });
      });

      call.on('end', () => {
        call.end();
      });
    }
  });

  return server;
}

if (process.argv[1] === __filename) {
  const port = process.env.PORT || '50051';
  const server = createGrpcServer();
  const address = `0.0.0.0:${port}`;

  server.bindAsync(address, grpc.ServerCredentials.createInsecure(), (err, boundPort) => {
    if (err) {
      console.error('[learn-grpc] Failed to bind server:', err);
      process.exit(1);
    }
    console.log(`[learn-grpc] Server listening on gRPC port ${boundPort} (${address})`);
    console.log('[learn-grpc] Protocol Buffers service: learning.v1.LearningService');
  });
}
