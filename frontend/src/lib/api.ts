// Typed facade over the generated Wails bindings.
import * as App from "../../wailsjs/go/main/App";
import { BrowserOpenURL, EventsOn, OnFileDrop, OnFileDropOff } from "../../wailsjs/runtime/runtime";
import type {
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
  openURL: (url: string) => BrowserOpenURL(url),

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

  // tools & runtime
  tools: () => call<ToolStatus[]>(App.GetTools()),
  downloadTool: (tool: string, thin = false) => App.DownloadTool(tool, thin),
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

export function on<T = unknown>(event: string, fn: (data: T) => void): () => void {
  return EventsOn(event, fn as (...data: unknown[]) => void);
}

/** Files dropped on elements styled with --wails-drop-target: drop. */
export function onFileDrop(fn: (paths: string[]) => void): () => void {
  OnFileDrop((_x, _y, paths) => fn(paths), true);
  return () => OnFileDropOff();
}

export const attachmentURL = (name: string) => `/attachments/${encodeURIComponent(name)}`;
export const imageURL = (file: string) => `/images/${encodeURIComponent(file)}`;
