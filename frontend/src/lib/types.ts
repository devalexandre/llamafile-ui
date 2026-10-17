export type Page = "chat" | "images" | "voice" | "documents" | "discover" | "library" | "settings";

export interface Hardware {
  os: string;
  arch: string;
  cpuName: string;
  cpuThreads: number;
  totalRamGb: number;
  availableRamGb: number;
  gpuName: string;
  gpuVendor: string;
  vramGb: number;
  unifiedMemory: boolean;
  backend: string;
  gpuBandwidthGbps: number;
  cpuBandwidthGbps: number;
  usableRamGb: number;
  usableVramGb: number;
  tier: string;
}

export interface Settings {
  host: string;
  port: number;
  llamafilePath: string;
  gpu: string;
  contextSize: number;
  threads: number;
  autoStart: boolean;
  theme: "system" | "light" | "dark";
  modelsDir: string;
  systemPrompt: string;
  temperature: number;
  lastModelId: string;
  autoTitle: boolean;
  voiceModelId: string;
  voiceLanguage: string;
  embedModelId: string;
  imageModelId: string;
  fastDecode: boolean;
  closeToTray: boolean;
  imageEngine: "auto" | "diffusionfile" | "sd-cpp";
  ttsVoiceId: string;
  ttsSpeed: number;
  autoSpeak: boolean;
}

export type FitLevel = "perfect" | "good" | "marginal" | "tight";

export interface Fit {
  quant: string;
  file: string;
  downloadBytes: number;
  requiredGb: number;
  mode: string;
  utilization: number;
  level: FitLevel;
  label: string;
  tps: number;
  realtimeFactor: number;
  secondsPerImage: number;
  speedLabel: string;
  qualityScore: number;
  speedScore: number;
  fitScore: number;
  contextScore: number;
  score: number;
  category: string;
  /** Speed measured on this machine for this model. */
  measured: boolean;
  /** Speed adjusted by measurements of other models. */
  calibrated: boolean;
  samples: number;
}

export interface ModelFile {
  quant?: string;
  file: string;
  url: string;
  size: number;
}

export interface ModelView {
  id: string;
  name: string;
  publisher: string;
  params_b: number;
  active_params_b: number;
  context: number;
  categories: string[];
  quality: Record<string, number>;
  runtime: "llamafile" | "whisperfile" | "diffusionfile" | "piper";
  repo: string;
  license: string;
  tags: string[];
  description: string;
  files: ModelFile[];
  mmproj?: ModelFile;
  components?: (ModelFile & { role: string })[];
  fit: Fit;
  installed: boolean;
  installedQuant: string;
  installedBytes: number;
  local: boolean;
  vision: boolean;
}

export interface Category {
  id: string;
  label: string;
  description: string;
}

export interface Download {
  key: string;
  label: string;
  received: number;
  total: number;
  progress: number;
  speed: number;
  eta: number;
  file: number;
  files: number;
}

export type RuntimeState = "missing" | "stopped" | "starting" | "running" | "error";

export interface RuntimeStatus {
  role: "chat" | "embed";
  thin: boolean;
  state: RuntimeState;
  message: string;
  error: string;
  binaryPath: string;
  version: string;
  modelId: string;
  modelName: string;
  host: string;
  port: number;
  baseUrl: string;
  needsRestart: boolean;
}

export interface Release {
  version: string;
  published: string;
  assets: Record<string, { url: string; size: number }>;
}

export interface Conversation {
  id: string;
  title: string;
  modelId: string;
  systemPrompt: string;
  pinned: boolean;
  collectionId: string;
  createdAt: number;
  updatedAt: number;
  preview: string;
  messageCount: number;
}

export interface Message {
  id: number;
  conversationId: string;
  role: "system" | "user" | "assistant";
  content: string;
  reasoning: string;
  modelId: string;
  attachments: string[];
  tps: number;
  tokens: number;
  error: string;
  sources: Source[];
  createdAt: number;
}

export interface ConversationDetail {
  conversation: Conversation;
  messages: Message[];
  generating: boolean;
  generatingImage: boolean;
}

// ---------------------------------------------------------------- tools

export interface ToolStatus {
  tool: "llamafile" | "whisperfile" | "diffusionfile" | "sd-cpp" | "piper";
  installed: boolean;
  path: string;
  version: string;
  thin: boolean;
  downloading: boolean;
  backends: string[];
  gpu: boolean;
  probed: boolean;
}

export interface BenchRecord {
  modelId: string;
  quant: string;
  kind: "llm" | "image" | "voice";
  mode: string;
  ratio: number;
  value: number;
  promptTps: number;
  samples: number;
  updatedAt: number;
}

export interface BenchResult {
  modelId: string;
  tps: number;
  promptTps: number;
  estimateTps: number;
  tokens: number;
  mode: string;
}

// ---------------------------------------------------------------- voice

export interface Transcript {
  id: number;
  name: string;
  source: "recording" | "file";
  modelId: string;
  language: string;
  durationMs: number;
  text: string;
  srt: string;
  createdAt: number;
}

// ------------------------------------------------------------ documents

export interface Collection {
  id: string;
  name: string;
  embedModel: string;
  documents: number;
  chunks: number;
  createdAt: number;
  updatedAt: number;
}

export interface DocumentItem {
  id: number;
  collectionId: string;
  name: string;
  path: string;
  size: number;
  chars: number;
  chunks: number;
  status: "queued" | "indexing" | "ready" | "error";
  error: string;
  progress: number;
  createdAt: number;
}

export interface Hit {
  documentId: number;
  document: string;
  chunk: number;
  text: string;
  score: number;
}

export interface Source {
  n: number;
  document: string;
  chunk: number;
  text: string;
  score: number;
}

// --------------------------------------------------------------- images

export interface ImageRequest {
  modelId: string;
  prompt: string;
  negative: string;
  width: number;
  height: number;
  steps: number;
  cfg: number;
  seed: number;
  sampler: string;
  fastDecode: boolean;
  /** Reference image to transform (attachment name); "" = text-to-image. */
  initImage: string;
  strength: number;
}

export interface ImageItem {
  id: number;
  prompt: string;
  negative: string;
  modelId: string;
  modelName: string;
  width: number;
  height: number;
  steps: number;
  cfg: number;
  seed: number;
  sampler: string;
  file: string;
  durationMs: number;
  createdAt: number;
  reference: string;
  strength: number;
}

export interface ImageProgress {
  jobId: string;
  phase: "loading" | "encoding" | "sampling" | "decoding" | "saving";
  step: number;
  steps: number;
  elapsed: number;
  eta: number;
}

export interface ImageDefaults {
  steps: number;
  cfg: number;
  width: number;
  height: number;
  sampler: string;
  negative: boolean;
  taesd: boolean;
}

/** Content handed to the chat from other pages ("send to chat"). */
export interface ChatDraft {
  text?: string;
  attachments?: string[];
  collectionId?: string;
}
