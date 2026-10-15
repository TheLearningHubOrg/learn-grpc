export interface RoadmapStage {
  stage_number: number;
  title: string;
  domain_focus: string;
  key_concepts: string[];
}

export const STAGES: RoadmapStage[] = [
  {
    stage_number: 1,
    title: 'Core Architecture, Protocol Buffers & HTTP/2 Transport',
    domain_focus: 'Protobuf TLV Wire Format, Varints, HTTP/2 Multiplexing, 5-Byte Framing',
    key_concepts: ['Protobuf wire format', 'Varints', 'HTTP/2 streams', '5-byte length prefix']
  },
  {
    stage_number: 2,
    title: 'The Four RPC Interaction Patterns',
    domain_focus: 'Unary, Server Streaming, Client Streaming, Bi-directional Duplex',
    key_concepts: ['Unary RPC', 'Server Streaming', 'Client Streaming', 'Bidi Streaming']
  },
  {
    stage_number: 3,
    title: 'Interceptors, Metadata, Deadlines & Cancellation',
    domain_focus: 'Middleware Chains, Metadata Headers, Deadline Propagation, Context',
    key_concepts: ['Unary Interceptors', 'Streaming Interceptors', 'Metadata -bin', 'Deadline timeout']
  },
  {
    stage_number: 4,
    title: 'Error Handling & Resilience',
    domain_focus: '16 Canonical Status Codes, Google Rich Error Model, Retries & Circuit Breakers',
    key_concepts: ['Canonical codes', 'google.rpc.Status', 'Hedging', 'Exponential backoff']
  },
  {
    stage_number: 5,
    title: 'High-Performance Load Balancing & Name Resolution',
    domain_focus: 'L4 vs L7 Balancing, Client-side balancing, Envoy Proxy, gRPC-Web',
    key_concepts: ['HTTP/2 connection pooling', 'Envoy bridge', 'gRPC-Web', 'Headless Kubernetes']
  },
  {
    stage_number: 6,
    title: 'Enterprise Security & Observability',
    domain_focus: 'mTLS, Token Auth, Reflection Protocol, Health Checking Probes',
    key_concepts: ['mTLS certificates', 'Server reflection', 'grpcurl', 'Health Checking v1']
  },
  {
    stage_number: 7,
    title: 'Production Handbook & Staff Q&A',
    domain_focus: '50 Staff-level scenarios, catastrophic failure modes, disaster recovery',
    key_concepts: ['Failure mitigation', 'Zero-downtime upgrades', 'Performance tuning']
  }
];
