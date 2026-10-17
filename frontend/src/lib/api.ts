// Typed facade over the generated Wails bindings.
import { Browser } from "@wailsio/runtime";
import * as App from "../../bindings/github.com/devalexandre/llamafile-ui/app";
import type {
  BenchRecord,
  BenchResult,
  Category,
  Collection,
  Conversation,
  ConversationDetail,
  DocumentItem,
  Download,
  Hardware,
  Hit,
  ImageDefaults,
  ImageItem,
  ImageRequest,
  ModelView,
  Release,
  RuntimeStatus,
  Settings,
  ToolStatus,
  Transcript,
} from "./types";

const call = <T>(p: Promise<unknown>) => p as Promise<T>;

export const api = {
  // system
  hardware: () => call<Hardware>(App.GetHardware()),
  refreshHardware: () => call<Hardware>(App.RefreshHardware()),
  diskFree: () => call<number>(App.GetDiskFreeGB()),
  platform: () => call<string>(App.GetPlatform()),
  settings: () => call<Settings>(App.GetSettings()),
  saveSettings: (s: Settings) => call<Settings>(App.SaveSettings(s as never)),
  resetServerSettings: () => call<Settings>(App.ResetServerSettings()),
  chooseModelsDir: () => call<string>(App.ChooseModelsDir()),
  openModelsFolder: () => App.OpenModelsFolder(),
  openDataFolder: () => App.OpenDataFolder(),
  openURL: (url: string) => Browser.OpenURL(url),

  // catalog
  categories: () => call<Category[]>(App.GetCategories()),
  recommended: (category: string) => call<ModelView[]>(App.GetRecommended(category)),
  topPicks: () => call<Record<string, ModelView>>(App.GetTopPicks()),
  installed: () => call<ModelView[]>(App.GetInstalled()),
  installedFor: (category: string) => call<ModelView[]>(App.GetInstalledFor(category)),
  downloads: () => call<Download[]>(App.GetDownloads()),
  downloadModel: (id: string, quant = "") => App.DownloadModel(id, quant),
  cancelDownload: (key: string) => App.CancelDownload(key),
  removeModel: (id: string) => App.RemoveModel(id),
  speak: (id: string, text: string) => App.Speak(id, text),
  stopSpeaking: () => App.StopSpeaking(),
  speakingId: () => call<string>(App.SpeakingID()),
  previewVoice: (id: string) => App.PreviewVoice(id),
  pickChatFiles: () => call<string[]>(App.PickChatFiles()),
  attachFiles: (paths: string[]) => call<string[]>(App.AttachFiles(paths)),
  transcribeForChat: (path: string) => call<string>(App.TranscribeForChat(path)),
  generateChatImage: (conversationId: string, modelId: string, text: string, reference = "") =>
    call<Conversation>(App.GenerateChatImage(conversationId, modelId, text, reference)),
  benchmarkModel: (id: string) => call<BenchResult>(App.BenchmarkModel(id)),
  benchmarks: () => call<BenchRecord[]>(App.GetBenchmarks()),
  resetBenchmarks: () => App.ResetBenchmarks(),

  // tools & runtime
  tools: () => call<ToolStatus[]>(App.GetTools()),
  downloadTool: (tool: string, thin = false) => App.DownloadTool(tool, thin),
  detectToolGPU: () => call<ToolStatus[]>(App.DetectToolGPU()),
  runtime: () => call<RuntimeStatus>(App.GetRuntimeStatus()),
  embedder: () => call<RuntimeStatus>(App.GetEmbedderStatus()),
  detectLlamafile: () => call<RuntimeStatus>(App.DetectLlamafile()),
  latestRelease: () => call<Release>(App.GetLatestRelease()),
  downloadLlamafile: (thin: boolean) => App.DownloadLlamafile(thin),
  chooseLlamafile: () => call<RuntimeStatus>(App.ChooseLlamafileBinary()),
  startModel: (id: string) => App.StartModel(id),
  stopServer: () => App.StopServer(),
  stopEmbedder: () => App.StopEmbedder(),
  restartServer: () => App.RestartServer(),
  logs: () => call<string[]>(App.GetLogs()),

  // chat
  conversations: (search = "") => call<Conversation[]>(App.ListConversations(search)),
  conversation: (id: string) => call<ConversationDetail>(App.GetConversation(id)),
  send: (conversationId: string, modelId: string, collectionId: string, text: string, attachments: string[]) =>
    call<Conversation>(App.SendMessage(conversationId, modelId, collectionId, text, attachments)),
  stop: (conversationId: string) => App.StopGeneration(conversationId),
  regenerate: (conversationId: string) => App.Regenerate(conversationId),
  edit: (conversationId: string, messageId: number, text: string) => App.EditMessage(conversationId, messageId, text),
  rename: (id: string, title: string) => App.RenameConversation(id, title),
  pin: (id: string, pinned: boolean) => App.PinConversation(id, pinned),
  setConversationModel: (id: string, modelId: string) => App.SetConversationModel(id, modelId),
  setConversationCollection: (id: string, collectionId: string) => App.SetConversationCollection(id, collectionId),
  setSystemPrompt: (id: string, prompt: string) => App.SetConversationSystemPrompt(id, prompt),
  deleteConversation: (id: string) => App.DeleteConversation(id),
  deleteAllConversations: () => App.DeleteAllConversations(),
  exportConversation: (id: string, format: "md" | "json") => call<string>(App.ExportConversation(id, format)),
  pickImages: () => call<string[]>(App.PickImages()),
  saveAttachment: (base64: string, ext: string) => call<string>(App.SaveAttachment(base64, ext)),

  // voice
  setVoiceModel: (id: string) => App.SetVoiceModel(id),
  setVoiceLanguage: (lang: string) => App.SetVoiceLanguage(lang),
  startRecording: () => App.StartRecording(),
  stopRecording: (save: boolean) => call<Transcript>(App.StopRecording(save)),
  cancelRecording: () => App.CancelRecording(),
  cancelTranscription: () => App.CancelTranscription(),
  pickAudioFiles: () => call<string[]>(App.PickAudioFiles()),
  transcribeFile: (path: string) => call<Transcript>(App.TranscribeFile(path)),
  transcripts: () => call<Transcript[]>(App.ListTranscripts()),
  deleteTranscript: (id: number) => App.DeleteTranscript(id),
  exportTranscript: (id: number, format: "txt" | "srt") => call<string>(App.ExportTranscript(id, format)),

  // documents
  setEmbedModel: (id: string) => App.SetEmbedModel(id),
  collections: () => call<Collection[]>(App.ListCollections()),
  createCollection: (name: string) => call<Collection>(App.CreateCollection(name)),
  renameCollection: (id: string, name: string) => App.RenameCollection(id, name),
  deleteCollection: (id: string) => App.DeleteCollection(id),
  reindexCollection: (id: string) => App.ReindexCollection(id),
  setCollectionModel: (id: string, model: string) => App.SetCollectionModel(id, model),
  documents: (collectionId: string) => call<DocumentItem[]>(App.ListDocuments(collectionId)),
  addDocuments: (collectionId: string) => call<DocumentItem[]>(App.AddDocuments(collectionId)),
  addDocumentPaths: (collectionId: string, paths: string[]) => call<DocumentItem[]>(App.AddDocumentPaths(collectionId, paths)),
  removeDocument: (id: number) => App.RemoveDocument(id),
  search: (collectionId: string, query: string) => call<Hit[]>(App.SearchCollection(collectionId, query)),

  // images
  imageDefaults: (modelId: string) => call<ImageDefaults>(App.GetImageDefaults(modelId)),
  estimateImage: (req: ImageRequest) => call<number>(App.EstimateImage(req as never)),
  generateImage: (req: ImageRequest) => call<string>(App.GenerateImage(req as never)),
  cancelImage: () => App.CancelImage(),
  imageJob: () => call<string>(App.GetImageJob()),
  images: () => call<ImageItem[]>(App.ListImages()),
  deleteImage: (id: number) => App.DeleteImage(id),
  saveImageAs: (id: number) => call<string>(App.SaveImageAs(id)),
  imageToAttachment: (id: number) => call<string>(App.ImageToAttachment(id)),
  enhancePrompt: (prompt: string) => call<string>(App.EnhanceImagePrompt(prompt)),
};

// ------------------------------------------------------------------ events
//
// Backend events arrive over HTTP long-polling (GET /_bus/events, served by
// the Go side) instead of Wails' script-injection events, which proved
// unreliable with WebKitGTK 6 once the app runs child processes.

type Listener = (data: unknown) => void;
const listeners = new Map<string, Set<Listener>>();

interface BusResponse {
  seq: number;
  events: { seq: number; name: string; data?: unknown }[];
}

async function pollEvents() {
  let after = -1;
  for (;;) {
    try {
      const res = await fetch(`/_bus/events?after=${after}`, { cache: "no-store" });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const body = (await res.json()) as BusResponse;
      for (const ev of body.events) {
        listeners.get(ev.name)?.forEach((fn) => {
          try {
            fn(ev.data);
          } catch (e) {
            console.error(`event handler for ${ev.name} failed`, e);
          }
        });
      }
      after = body.seq;
    } catch {
      await new Promise((r) => setTimeout(r, 1000));
    }
  }
}
pollEvents();

export function on<T = unknown>(event: string, fn: (data: T) => void): () => void {
  const set = listeners.get(event) ?? new Set<Listener>();
  set.add(fn as Listener);
  listeners.set(event, set);
  return () => {
    set.delete(fn as Listener);
  };
}

/** Files dropped on elements marked with the data-file-drop-target attribute. */
export function onFileDrop(fn: (paths: string[]) => void): () => void {
  return on<string[]>("files:dropped", (paths) => fn(paths ?? []));
}

export const attachmentURL = (name: string) => `/attachments/${encodeURIComponent(name)}`;
export const imageURL = (file: string) => `/images/${encodeURIComponent(file)}`;
