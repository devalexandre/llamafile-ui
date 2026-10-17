import clsx from "clsx";
import {
  ChevronDown,
  Copy,
  Dices,
  Download,
  ImagePlus,
  ImageUp,
  Loader2,
  MessageSquare,
  RotateCcw,
  Sparkles,
  Square,
  Trash2,
  Wand2,
  X,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { ModelGate, ModelSelect, ToolGate } from "../components/Gates";
import { Button, EmptyState, IconButton, Progress, Segmented, Sheet, Switch } from "../components/ui";
import { api, attachmentURL, imageURL, on, onFileDrop } from "../lib/api";
import { duration } from "../lib/format";
import { errorText, useApp } from "../lib/store";
import type { ImageDefaults, ImageItem, ImageProgress, ImageRequest } from "../lib/types";

const sizes = [
  { value: "square", label: "Quadrado", w: 1, h: 1 },
  { value: "portrait", label: "Retrato", w: 2, h: 3 },
  { value: "landscape", label: "Paisagem", w: 3, h: 2 },
] as const;

export const phaseLabel: Record<ImageProgress["phase"], string> = {
  loading: "Carregando o modelo…",
  encoding: "Entendendo o prompt…",
  sampling: "Gerando a imagem…",
  decoding: "Finalizando os detalhes…",
  saving: "Salvando…",
};

const examples = [
  "Um farol em uma ilha rochosa ao pôr do sol, pintura a óleo",
  "Raposa vermelha na neve, fotografia, luz suave",
  "Cidade futurista com neon à noite, ilustração digital",
  "Xícara de café em uma mesa de madeira, foto de produto, minimalista",
];

function dims(shape: string, base: number) {
  const s = sizes.find((x) => x.value === shape) ?? sizes[0];
  const scale = base / Math.max(s.w, s.h);
  return { width: Math.round((s.w * scale) / 64) * 64, height: Math.round((s.h * scale) / 64) * 64 };
}

function Lightbox({
  img,
  onClose,
  onReuse,
  onReference,
}: {
  img: ImageItem;
  onClose: () => void;
  onReuse: (img: ImageItem) => void;
  onReference: (img: ImageItem) => void;
}) {
  const app = useApp();
  const act = (fn: () => Promise<unknown>) => fn().catch((e) => app.toast(errorText(e), "error"));
  return (
    <Sheet open onClose={onClose} width={860}>
      <div className="flex gap-5">
        <img src={imageURL(img.file)} className="max-h-[70vh] w-[520px] rounded-lg border border-line object-contain" />
        <div className="flex min-w-0 flex-1 flex-col">
          <div className="flex items-start justify-between gap-2">
            <div className="text-[13px] font-semibold">Detalhes</div>
            <IconButton title="Fechar" onClick={onClose}>
              <X className="size-4" />
            </IconButton>
          </div>
          <p className="selectable mt-2 text-[13px] leading-relaxed">{img.prompt}</p>
          {img.negative && <p className="selectable mt-1 text-[12px] text-muted">Evitar: {img.negative}</p>}
          <dl className="mt-3 grid grid-cols-2 gap-x-3 gap-y-1 text-[12px]">
            <dt className="text-muted">Modelo</dt>
            <dd>{img.modelName}</dd>
            <dt className="text-muted">Tamanho</dt>
            <dd>
              {img.width}×{img.height}
            </dd>
            <dt className="text-muted">Passos · CFG</dt>
            <dd>
              {img.steps} · {img.cfg}
            </dd>
            <dt className="text-muted">Seed</dt>
            <dd className="selectable">{img.seed}</dd>
            <dt className="text-muted">Tempo</dt>
            <dd>{duration(img.durationMs / 1000)}</dd>
          </dl>
          {img.reference && (
            <div className="mt-3">
              <div className="mb-1 text-[12px] text-muted">Feita a partir de (intensidade {Math.round(img.strength * 100)}%)</div>
              <img src={imageURL(img.reference)} className="max-h-24 rounded-md border border-line object-contain" />
            </div>
          )}
          <div className="mt-auto flex flex-wrap gap-2 pt-4">
            <Button size="sm" icon={<RotateCcw className="size-3.5" />} onClick={() => onReuse(img)}>
              Reutilizar
            </Button>
            <Button size="sm" icon={<ImageUp className="size-3.5" />} onClick={() => onReference(img)}>
              Usar como referência
            </Button>
            <Button
              size="sm"
              icon={<Copy className="size-3.5" />}
              onClick={() => {
                navigator.clipboard.writeText(img.prompt);
                app.toast("Prompt copiado.");
              }}
            >
              Copiar prompt
            </Button>
            <Button
              size="sm"
              icon={<Download className="size-3.5" />}
              onClick={() => act(async () => (await api.saveImageAs(img.id)) && app.toast("Imagem salva.", "success"))}
            >
              Salvar como…
            </Button>
            <Button
              size="sm"
              icon={<MessageSquare className="size-3.5" />}
              onClick={() =>
                act(async () => {
                  const name = await api.imageToAttachment(img.id);
                  const vision = app.chatModels.find((m) => m.vision);
                  if (vision) await app.saveSettings({ lastModelId: vision.id });
                  else app.toast("Baixe um modelo de Visão para conversar sobre imagens.");
                  app.newChat({ text: "Descreva esta imagem.", attachments: [name] });
                })
              }
            >
              Perguntar no chat
            </Button>
            <Button
              size="sm"
              variant="danger"
              icon={<Trash2 className="size-3.5" />}
              onClick={() =>
                act(async () => {
                  await api.deleteImage(img.id);
                  onClose();
                })
              }
            >
              Excluir
            </Button>
          </div>
        </div>
      </div>
    </Sheet>
  );
}

function Generator() {
  const app = useApp();
  const models = app.imageModels;
  const [modelId, setModelId] = useState(models[0]?.id ?? "");
  const [defaults, setDefaults] = useState<ImageDefaults | null>(null);
  const [prompt, setPrompt] = useState("");
  const [negative, setNegative] = useState("");
  const [shape, setShape] = useState<string>("square");
  const [base, setBase] = useState(512);
  const [steps, setSteps] = useState(1);
  const [cfg, setCfg] = useState(1);
  const [seed, setSeed] = useState(-1);
  const [fastDecode, setFastDecode] = useState(app.settings?.fastDecode ?? true);
  const [advanced, setAdvanced] = useState(false);
  const [enhancing, setEnhancing] = useState(false);
  const [estimate, setEstimate] = useState(0);
  const [job, setJob] = useState("");
  const [progress, setProgress] = useState<ImageProgress | null>(null);
  const [images, setImages] = useState<ImageItem[]>([]);
  const [open, setOpen] = useState<ImageItem | null>(null);
  // Reference image (img2img): an attachment name and how much it may change.
  const [reference, setReference] = useState("");
  const [strength, setStrength] = useState(0.6);
  useEffect(
    () =>
      onFileDrop(async (paths) => {
        const img = paths.find((p) => /\.(png|jpe?g|webp|gif)$/i.test(p));
        if (!img) return app.toast("Arraste uma imagem PNG, JPG, WebP ou GIF.");
        try {
          const [name] = await api.attachFiles([img]);
          if (name) setReference(name);
        } catch (e) {
          app.toast(errorText(e), "error");
        }
      }),
    [app],
  );

  const model = models.find((m) => m.id === modelId);
  const size = dims(shape, base);

  const loadImages = useCallback(() => api.images().then((l) => setImages(l ?? [])), []);
  useEffect(() => {
    loadImages();
    api.imageJob().then((j) => j && setJob(j));
  }, [loadImages]);

  useEffect(() => {
    if (!models.some((m) => m.id === modelId)) setModelId(models[0]?.id ?? "");
  }, [models, modelId]);

  // Apply the model's recommended settings when it changes.
  useEffect(() => {
    if (!modelId) return;
    api.imageDefaults(modelId).then((d) => {
      setDefaults(d);
      setSteps(d.steps);
      setCfg(d.cfg);
      setBase(Math.max(d.width, d.height));
    });
  }, [modelId]);

  const request = useMemo<ImageRequest>(
    () => ({
      modelId,
      prompt,
      negative,
      ...size,
      steps,
      cfg,
      seed,
      sampler: defaults?.sampler ?? "",
      fastDecode,
      initImage: reference,
      strength: reference ? strength : 0,
    }),
    [modelId, prompt, negative, size.width, size.height, steps, cfg, seed, defaults?.sampler, fastDecode, reference, strength], // eslint-disable-line react-hooks/exhaustive-deps
  );

  useEffect(() => {
    if (modelId) api.estimateImage(request).then(setEstimate);
  }, [request, modelId]);

  useEffect(() => {
    const offs = [
      on<ImageProgress>("image:progress", (p) => {
        setJob(p.jobId);
        setProgress(p);
      }),
      on<{ jobId: string; image: ImageItem }>("image:done", ({ image }) => {
        setJob("");
        setProgress(null);
        setImages((l) => [image, ...l]);
      }),
      on<{ jobId: string; error: string }>("image:error", ({ error }) => {
        setJob("");
        setProgress(null);
        if (error) app.toast(error, "error");
      }),
    ];
    return () => offs.forEach((off) => off());
  }, [app]);

  const generate = async () => {
    try {
      const id = await api.generateImage(request);
      setJob(id);
      setProgress({ jobId: id, phase: "loading", step: 0, steps, elapsed: 0, eta: estimate });
    } catch (e) {
      app.toast(errorText(e), "error");
    }
  };

  const reuse = (img: ImageItem) => {
    setModelId(img.modelId);
    setPrompt(img.prompt);
    setNegative(img.negative);
    setSeed(img.seed);
    setOpen(null);
    setTimeout(() => {
      setSteps(img.steps);
      setCfg(img.cfg);
    }, 300);
  };

  const pct = progress
    ? progress.phase === "sampling" && progress.steps > 0
      ? 0.1 + 0.75 * (progress.step / progress.steps)
      : { loading: 0.03, encoding: 0.08, sampling: 0.1, decoding: 0.88, saving: 0.98 }[progress.phase]
    : 0;

  return (
    <div className="flex min-h-0 flex-1">
      {/* Controls */}
      <aside className="flex w-[340px] shrink-0 flex-col gap-4 overflow-y-auto border-r border-line p-5">
        <div>
          <div className="mb-1.5 text-[12px] font-semibold text-muted">Modelo</div>
          <ModelSelect models={models} value={modelId} onChange={setModelId} />
        </div>
        <div>
          <div className="mb-1.5 text-[12px] font-semibold text-muted">Descreva a imagem</div>
          <textarea
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && (e.metaKey || e.ctrlKey) && !job) generate();
            }}
            rows={5}
            placeholder="Ex.: uma raposa vermelha na neve, fotografia"
            className="w-full resize-none rounded-[10px] border border-line bg-card px-3 py-2 text-[13px] outline-none focus:border-accent focus:ring-3 focus:ring-accent-soft"
          />
          <div className="mt-1.5 flex items-center justify-between gap-2">
            <p className="text-[11px] text-faint">Prompts detalhados em inglês funcionam melhor.</p>
            <Button
              size="sm"
              variant="plain"
              icon={<Sparkles className="size-3.5" />}
              loading={enhancing}
              disabled={!prompt.trim() || app.chatModels.length === 0}
              title={app.chatModels.length === 0 ? "Precisa de um modelo de chat instalado" : "Traduz e detalha o prompt com o modelo de chat"}
              onClick={async () => {
                setEnhancing(true);
                try {
                  setPrompt(await api.enhancePrompt(prompt));
                } catch (e) {
                  app.toast(errorText(e), "error");
                } finally {
                  setEnhancing(false);
                }
              }}
            >
              Melhorar prompt
            </Button>
          </div>
        </div>
        <div>
          <div className="mb-1.5 text-[12px] font-semibold text-muted">Imagem de referência (opcional)</div>
          {reference ? (
            <div className="rounded-[10px] border border-line bg-card p-2.5">
              <div className="flex items-start gap-2.5">
                <img src={attachmentURL(reference)} className="size-16 rounded-md border border-line object-cover" />
                <div className="min-w-0 flex-1 text-[11.5px] text-muted">
                  O modelo vai alterar esta imagem seguindo a descrição, mantendo a composição e a proporção.
                </div>
                <IconButton title="Remover referência" onClick={() => setReference("")}>
                  <X className="size-4" />
                </IconButton>
              </div>
              <label className="mt-2 flex items-center gap-2 text-[12px]">
                <span className="w-20 text-muted">Intensidade</span>
                <input
                  type="range"
                  min={0.1}
                  max={1}
                  step={0.05}
                  value={strength}
                  onChange={(e) => setStrength(Number(e.target.value))}
                  className="flex-1 accent-[var(--accent)]"
                />
                <span className="w-9 text-right tabular-nums">{Math.round(strength * 100)}%</span>
              </label>
              <p className="mt-1 text-[11px] text-faint">Baixa: pequenos ajustes. Alta: muda quase tudo.</p>
            </div>
          ) : (
            <div
              data-file-drop-target
              className="flex items-center gap-2 rounded-[10px] border border-dashed border-line px-3 py-2.5 text-[12px] text-muted [&.file-drop-target-active]:border-accent [&.file-drop-target-active]:bg-accent-soft"
            >
              <ImageUp className="size-4 shrink-0" />
              <span className="flex-1">Arraste uma imagem ou</span>
              <Button
                size="sm"
                variant="plain"
                onClick={async () => {
                  try {
                    const names = await api.pickImages();
                    if (names?.[0]) setReference(names[0]);
                  } catch (e) {
                    app.toast(errorText(e), "error");
                  }
                }}
              >
                Escolher…
              </Button>
            </div>
          )}
        </div>
        {defaults?.negative && (
          <div>
            <div className="mb-1.5 text-[12px] font-semibold text-muted">Evitar (prompt negativo)</div>
            <input
              value={negative}
              onChange={(e) => setNegative(e.target.value)}
              placeholder="borrado, deformado, texto"
              className="h-[30px] w-full rounded-[8px] border border-line bg-card px-2.5 text-[13px] outline-none focus:border-accent"
            />
          </div>
        )}
        <div className={clsx(reference && "hidden")}>
          <div className="mb-1.5 text-[12px] font-semibold text-muted">Formato</div>
          <Segmented value={shape} onChange={setShape} options={sizes.map((s) => ({ value: s.value, label: s.label }))} />
          <div className="mt-2 flex items-center gap-2 text-[12px] text-muted">
            Tamanho
            <Segmented
              value={String(base)}
              onChange={(v) => setBase(Number(v))}
              options={[512, 768, 1024].map((n) => ({ value: String(n), label: String(n) }))}
            />
            <span className="tabular-nums">
              {size.width}×{size.height}
            </span>
          </div>
        </div>

        <button onClick={() => setAdvanced((a) => !a)} className="flex items-center gap-1 text-[12px] font-semibold text-muted hover:text-fg">
          <ChevronDown className={clsx("size-3.5 transition-transform", !advanced && "-rotate-90")} /> Avançado
        </button>
        {advanced && (
          <div className="-mt-2 space-y-3 rounded-lg bg-field p-3 text-[12px]">
            <label className="flex items-center gap-2">
              <span className="w-20 text-muted">Passos</span>
              <input type="range" min={1} max={50} value={steps} onChange={(e) => setSteps(Number(e.target.value))} className="flex-1 accent-[var(--accent)]" />
              <span className="w-6 text-right tabular-nums">{steps}</span>
            </label>
            <label className="flex items-center gap-2">
              <span className="w-20 text-muted">CFG</span>
              <input type="range" min={1} max={12} step={0.5} value={cfg} onChange={(e) => setCfg(Number(e.target.value))} className="flex-1 accent-[var(--accent)]" />
              <span className="w-6 text-right tabular-nums">{cfg}</span>
            </label>
            <label className="flex items-center gap-2">
              <span className="w-20 text-muted">Seed</span>
              <input
                value={seed < 0 ? "" : seed}
                placeholder="aleatória"
                onChange={(e) => setSeed(e.target.value === "" ? -1 : Number(e.target.value.replace(/\D/g, "")))}
                className="h-[26px] w-full rounded-md border border-line bg-card px-2 outline-none"
              />
              <IconButton title="Seed aleatória" onClick={() => setSeed(-1)}>
                <Dices className="size-4" />
              </IconButton>
            </label>
            {defaults?.taesd && (
              <label className="flex items-center justify-between gap-2">
                <span>
                  Decodificação rápida
                  <span className="block text-[11px] text-faint">Muito mais rápido na CPU, com detalhes um pouco mais suaves.</span>
                </span>
                <Switch checked={fastDecode} onChange={setFastDecode} />
              </label>
            )}
            {defaults && (
              <button
                className="text-accent hover:underline"
                onClick={() => {
                  setSteps(defaults.steps);
                  setCfg(defaults.cfg);
                }}
              >
                Restaurar recomendados ({defaults.steps} passos, CFG {defaults.cfg})
              </button>
            )}
          </div>
        )}

        <div className="mt-auto">
          {job ? (
            <div className="rounded-xl border border-line bg-card p-3">
              <div className="mb-2 flex items-center justify-between text-[12.5px]">
                <span className="flex items-center gap-1.5 font-medium">
                  <Loader2 className="size-3.5 animate-spin text-accent" />
                  {progress ? phaseLabel[progress.phase] : "Iniciando…"}
                </span>
                {progress?.phase === "sampling" && progress.steps > 1 && (
                  <span className="text-muted tabular-nums">
                    {progress.step}/{progress.steps}
                  </span>
                )}
              </div>
              <Progress value={pct} />
              <div className="mt-2 flex items-center justify-between text-[11.5px] text-muted">
                <span>
                  {duration(progress?.elapsed ?? 0)}
                  {progress && progress.eta > 1 && ` · faltam ~${duration(progress.eta)}`}
                </span>
                <Button size="sm" icon={<Square className="size-3" />} onClick={() => api.cancelImage()}>
                  Cancelar
                </Button>
              </div>
            </div>
          ) : (
            <>
              <Button variant="primary" className="w-full" icon={<Wand2 className="size-4" />} disabled={!prompt.trim() || !modelId} onClick={generate}>
                {reference ? "Transformar imagem" : "Gerar imagem"}
              </Button>
              <p className="mt-1.5 text-center text-[11px] text-muted">
                {estimate > 0 && `Estimativa: ~${duration(estimate)} neste computador · `}Ctrl+Enter
              </p>
            </>
          )}
        </div>
      </aside>

      {/* Gallery */}
      <div className="min-w-0 flex-1 overflow-y-auto p-5">
        {images.length === 0 && !job ? (
          <EmptyState
            icon={<ImagePlus className="size-10" />}
            title="Suas imagens aparecerão aqui"
            text="Tudo é gerado no seu computador. Experimente um destes:"
            action={
              <div className="flex max-w-md flex-wrap justify-center gap-2">
                {examples.map((e) => (
                  <button key={e} onClick={() => setPrompt(e)} className="rounded-full border border-line bg-card px-3 py-1 text-[12px] hover:bg-hover">
                    {e}
                  </button>
                ))}
              </div>
            }
          />
        ) : (
          <div className="grid grid-cols-[repeat(auto-fill,minmax(200px,1fr))] gap-3">
            {job && (
              <div className="flex aspect-square flex-col items-center justify-center gap-2 rounded-xl border border-dashed border-line bg-field text-[12px] text-muted">
                <Sparkles className="size-6 animate-pulse text-accent" />
                {progress ? phaseLabel[progress.phase] : "Iniciando…"}
              </div>
            )}
            {images.map((img) => (
              <div key={img.id} className="group relative overflow-hidden rounded-xl border border-line bg-field">
                <button onClick={() => setOpen(img)} className="block w-full">
                  <img src={imageURL(img.file)} loading="lazy" className="aspect-square w-full object-cover transition-transform group-hover:scale-[1.02]" />
                  <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/70 to-transparent px-2.5 pt-6 pb-2 text-left text-[11.5px] text-white opacity-0 transition-opacity group-hover:opacity-100">
                    <div className="line-clamp-2">{img.prompt}</div>
                  </div>
                </button>
                <button
                  title="Excluir imagem"
                  onClick={async () => {
                    if (!confirm("Excluir esta imagem?")) return;
                    try {
                      await api.deleteImage(img.id);
                      loadImages();
                    } catch (e) {
                      app.toast(errorText(e), "error");
                    }
                  }}
                  className="absolute top-2 right-2 rounded-md bg-black/55 p-1.5 text-white opacity-0 transition-opacity group-hover:opacity-100 hover:bg-bad"
                >
                  <Trash2 className="size-3.5" />
                </button>
              </div>
            ))}
          </div>
        )}
      </div>

      {open && (
        <Lightbox
          img={open}
          onClose={() => {
            setOpen(null);
            loadImages();
          }}
          onReuse={reuse}
          onReference={async (img) => {
            try {
              setReference(await api.imageToAttachment(img.id));
              setOpen(null);
            } catch (e) {
              app.toast(errorText(e), "error");
            }
          }}
        />
      )}
    </div>
  );
}

export function ImagesPage() {
  const app = useApp();
  return (
    <div className="flex h-full min-w-0 flex-1 flex-col bg-bg">
      <header className="drag flex h-[52px] shrink-0 items-center border-b border-line px-6">
        <h1 className="text-[14px] font-semibold">Imagens</h1>
      </header>
      <ToolGate
        tool="diffusionfile"
        alternative={{ tool: "sd-cpp", label: "Baixar versão com GPU" }}
        icon={<ImagePlus className="size-6" />}
      >
        <ModelGate category="image" models={app.imageModels} icon={<ImagePlus className="size-6" />}>
          <Generator />
        </ModelGate>
      </ToolGate>
    </div>
  );
}
