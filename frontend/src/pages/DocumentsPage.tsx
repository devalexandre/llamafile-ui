import clsx from "clsx";
import {
  AlertCircle,
  BookOpen,
  CheckCircle2,
  FilePlus2,
  FileText,
  FolderPlus,
  Loader2,
  MessageSquare,
  MoreHorizontal,
  Pencil,
  RefreshCw,
  Search,
  Trash2,
} from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { ModelGate } from "../components/Gates";
import { Button, EmptyState, IconButton, Menu, Progress, Sheet, Tag } from "../components/ui";
import { api, on, onFileDrop } from "../lib/api";
import { bytes } from "../lib/format";
import { errorText, useApp } from "../lib/store";
import type { Collection, DocumentItem, Hit } from "../lib/types";
import { t } from "../i18n";

function NameSheet({ title, initial, onSubmit, onClose }: { title: string; initial: string; onSubmit: (v: string) => void; onClose: () => void }) {
  const [value, setValue] = useState(initial);
  return (
    <Sheet open onClose={onClose}>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          onSubmit(value);
        }}
      >
        <div className="mb-3 text-[14px] font-semibold">{title}</div>
        <input
          autoFocus
          value={value}
          onChange={(e) => setValue(e.target.value)}
          placeholder={t("Ex.: Contratos, Manual do produto, Artigos…")}
          className="mb-4 h-[30px] w-full rounded-[7px] border border-line bg-card px-2.5 outline-none focus:border-accent focus:ring-3 focus:ring-accent-soft"
        />
        <div className="flex justify-end gap-2">
          <Button type="button" onClick={onClose}>
            {t("Cancelar")}
          </Button>
          <Button type="submit" variant="primary" disabled={!value.trim()}>
            {t("Salvar")}
          </Button>
        </div>
      </form>
    </Sheet>
  );
}

function DocumentRow({ d }: { d: DocumentItem }) {
  return (
    <div className="flex items-center gap-3 px-4 py-2.5">
      <FileText className="size-4 shrink-0 text-muted" />
      <div className="min-w-0 flex-1">
        <div className="truncate text-[13px]" title={d.path}>
          {d.name}
        </div>
        <div className="text-[11.5px] text-muted">
          {bytes(d.size)}
          {d.chunks > 0 && ` · ${d.chunks === 1 ? t("{n} trecho", { n: d.chunks }) : t("{n} trechos", { n: d.chunks })}`}
          {d.status === "error" && <span className="text-bad"> · {d.error}</span>}
        </div>
      </div>
      <div className="w-[160px] shrink-0">
        {d.status === "ready" && (
          <span className="flex items-center justify-end gap-1 text-[12px] text-ok">
            <CheckCircle2 className="size-3.5" /> {t("Indexado")}
          </span>
        )}
        {d.status === "queued" && <span className="block text-right text-[12px] text-muted">{t("Na fila")}</span>}
        {d.status === "indexing" && (
          <div>
            <div className="mb-1 flex items-center justify-end gap-1 text-[11.5px] text-muted">
              <Loader2 className="size-3 animate-spin" /> {t("Indexando {pct}%", { pct: Math.round(d.progress * 100) })}
            </div>
            <Progress value={d.progress} />
          </div>
        )}
        {d.status === "error" && (
          <span className="flex items-center justify-end gap-1 text-[12px] text-bad">
            <AlertCircle className="size-3.5" /> {t("Erro")}
          </span>
        )}
      </div>
      <IconButton title={t("Remover da coleção")} onClick={() => api.removeDocument(d.id)}>
        <Trash2 className="size-3.5" />
      </IconButton>
    </div>
  );
}

function CollectionView({ c }: { c: Collection }) {
  const app = useApp();
  const [docs, setDocs] = useState<DocumentItem[]>([]);
  const [query, setQuery] = useState("");
  const [hits, setHits] = useState<Hit[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [renaming, setRenaming] = useState(false);

  const load = useCallback(() => api.documents(c.id).then((l) => setDocs(l ?? [])), [c.id]);
  useEffect(() => {
    load();
    setHits(null);
  }, [load]);

  useEffect(() => {
    const offs = [
      on<DocumentItem>("docs:document", (d) => {
        if (d.collectionId !== c.id) return;
        setDocs((list) => (list.some((x) => x.id === d.id) ? list.map((x) => (x.id === d.id ? d : x)) : [...list, d]));
      }),
      on("docs:changed", load),
    ];
    return () => offs.forEach((off) => off());
  }, [c.id, load]);

  const add = async (fn: () => Promise<DocumentItem[]>) => {
    try {
      await fn();
    } catch (e) {
      app.toast(errorText(e), "error");
    }
    load();
  };
  useEffect(() => onFileDrop((paths) => add(() => api.addDocumentPaths(c.id, paths))), [c.id]); // eslint-disable-line react-hooks/exhaustive-deps

  const model = app.installed.find((m) => m.id === c.embedModel);
  const pending = docs.filter((d) => d.status === "queued" || d.status === "indexing").length;
  const ready = docs.filter((d) => d.status === "ready").length;

  const runSearch = async () => {
    if (!query.trim()) return;
    setSearching(true);
    try {
      setHits(await api.search(c.id, query));
    } catch (e) {
      app.toast(errorText(e), "error");
    } finally {
      setSearching(false);
    }
  };

  return (
    <div className="min-w-0 flex-1 overflow-y-auto">
      <div className="mx-auto max-w-[820px] px-6 py-5">
        <div className="mb-4 flex items-start gap-3">
          <div className="min-w-0 flex-1">
            <h2 className="truncate text-[18px] font-semibold">{c.name}</h2>
            <div className="mt-0.5 flex flex-wrap items-center gap-1.5 text-[12px] text-muted">
              <span>
                {docs.length === 1 ? t("{n} documento", { n: docs.length }) : t("{n} documentos", { n: docs.length })}
              </span>
              <span>·</span>
              <span>{t("Embeddings:")}</span>
              <Tag>{model?.name ?? c.embedModel}</Tag>
              {!model && <span className="text-bad">{t("modelo não instalado")}</span>}
              {pending > 0 && <span>· {t("indexando {n}…", { n: pending })}</span>}
            </div>
          </div>
          <Button icon={<FilePlus2 className="size-3.5" />} onClick={() => add(() => api.addDocuments(c.id))}>
            {t("Adicionar arquivos")}
          </Button>
          <Button
            variant="primary"
            icon={<MessageSquare className="size-3.5" />}
            disabled={ready === 0}
            onClick={() => app.newChat({ collectionId: c.id })}
          >
            {t("Conversar")}
          </Button>
          <Menu
            trigger={(toggle) => (
              <IconButton title={t("Mais")} onClick={toggle}>
                <MoreHorizontal className="size-4" />
              </IconButton>
            )}
            items={[
              { label: t("Renomear"), icon: <Pencil className="size-3.5" />, onClick: () => setRenaming(true) },
              { label: t("Reindexar tudo"), icon: <RefreshCw className="size-3.5" />, onClick: () => api.reindexCollection(c.id) },
              ...app.embedModels
                .filter((m) => m.id !== c.embedModel)
                .map((m) => ({
                  label: t("Usar {name}", { name: m.name }),
                  icon: <RefreshCw className="size-3.5" />,
                  onClick: () => {
                    if (confirm(t("Trocar o modelo de embeddings para {name}? Todos os documentos serão reindexados.", { name: m.name })))
                      api.setCollectionModel(c.id, m.id);
                  },
                })),
              "separator",
              {
                label: t("Excluir coleção"),
                icon: <Trash2 className="size-3.5" />,
                danger: true,
                onClick: () => {
                  if (confirm(t("Excluir a coleção “{name}”? Os arquivos originais não são apagados.", { name: c.name }))) api.deleteCollection(c.id);
                },
              },
            ]}
          />
        </div>

        <div
          data-file-drop-target
          className="overflow-hidden rounded-[12px] border border-line bg-card [&.file-drop-target-active]:border-accent [&.file-drop-target-active]:bg-accent-soft"
        >
          {docs.length === 0 ? (
            <div className="flex flex-col items-center px-6 py-10 text-center">
              <FilePlus2 className="mb-2 size-8 text-faint" />
              <div className="text-[13.5px] font-semibold">{t("Arraste arquivos para cá")}</div>
              <p className="mt-1 max-w-sm text-[12.5px] text-muted">
                {t("PDF, Word (.docx), Markdown, texto, HTML e código. O conteúdo é indexado localmente e nunca sai do seu computador.")}
              </p>
              <Button className="mt-3" size="sm" icon={<FilePlus2 className="size-3.5" />} onClick={() => add(() => api.addDocuments(c.id))}>
                {t("Escolher arquivos…")}
              </Button>
            </div>
          ) : (
            <div className="divide-y divide-line">
              {docs.map((d) => (
                <DocumentRow key={d.id} d={d} />
              ))}
            </div>
          )}
        </div>

        {ready > 0 && (
          <section className="mt-6">
            <h3 className="mb-1.5 px-1 text-[12px] font-semibold text-muted">{t("Testar a busca")}</h3>
            <div className="flex gap-2">
              <div className="flex h-[30px] flex-1 items-center gap-1.5 rounded-[8px] border border-line bg-card px-2.5 focus-within:border-accent">
                <Search className="size-3.5 text-muted" />
                <input
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && runSearch()}
                  placeholder={t("Faça uma pergunta e veja os trechos encontrados")}
                  className="w-full bg-transparent text-[13px] outline-none"
                />
              </div>
              <Button loading={searching} onClick={runSearch}>
                {t("Buscar")}
              </Button>
            </div>
            {hits && (
              <div className="mt-3 space-y-2">
                {hits.length === 0 && <p className="text-[12.5px] text-muted">{t("Nada encontrado.")}</p>}
                {hits.map((h, i) => (
                  <div key={i} className="rounded-lg border border-line bg-card p-3">
                    <div className="mb-1 flex items-center gap-2 text-[11.5px]">
                      <span className="font-semibold">{h.document}</span>
                      <span className="text-muted">{t("trecho {n}", { n: h.chunk + 1 })}</span>
                      <span className={clsx("ml-auto tabular-nums", h.score > 0.5 ? "text-ok" : "text-muted")}>
                        {t("{pct}% similar", { pct: Math.round(h.score * 100) })}
                      </span>
                    </div>
                    <p className="selectable line-clamp-4 text-[12.5px] whitespace-pre-wrap text-fg/85">{h.text}</p>
                  </div>
                ))}
              </div>
            )}
          </section>
        )}
      </div>
      {renaming && (
        <NameSheet
          title={t("Renomear coleção")}
          initial={c.name}
          onClose={() => setRenaming(false)}
          onSubmit={async (v) => {
            await api.renameCollection(c.id, v);
            setRenaming(false);
          }}
        />
      )}
    </div>
  );
}

function Collections() {
  const app = useApp();
  const [selected, setSelected] = useState("");
  const [creating, setCreating] = useState(false);
  const current = app.collections.find((c) => c.id === selected) ?? app.collections[0];

  return (
    <div className="flex min-h-0 flex-1">
      <aside className="flex w-[240px] shrink-0 flex-col border-r border-line p-3">
        <Button className="mb-3" icon={<FolderPlus className="size-3.5" />} onClick={() => setCreating(true)}>
          {t("Nova coleção")}
        </Button>
        <div className="space-y-px overflow-y-auto">
          {app.collections.map((c) => (
            <button
              key={c.id}
              onClick={() => setSelected(c.id)}
              className={clsx(
                "flex w-full items-center gap-2 rounded-[7px] px-2.5 py-1.5 text-left text-[13px]",
                current?.id === c.id ? "bg-accent text-on-accent" : "hover:bg-hover",
              )}
            >
              <BookOpen className="size-3.5 shrink-0 opacity-70" />
              <span className="flex-1 truncate">{c.name}</span>
              <span className={clsx("text-[11px]", current?.id === c.id ? "text-on-accent/75" : "text-muted")}>{c.documents}</span>
            </button>
          ))}
        </div>
      </aside>
      {current ? (
        <CollectionView key={current.id} c={current} />
      ) : (
        <div className="flex flex-1 items-center justify-center">
          <EmptyState
            icon={<BookOpen className="size-10" />}
            title={t("Converse com seus documentos")}
            text={t("Crie uma coleção, adicione PDFs, textos ou código e faça perguntas no chat — as respostas citam os trechos usados.")}
            action={
              <Button variant="primary" icon={<FolderPlus className="size-3.5" />} onClick={() => setCreating(true)}>
                {t("Criar coleção")}
              </Button>
            }
          />
        </div>
      )}
      {creating && (
        <NameSheet
          title={t("Nova coleção")}
          initial=""
          onClose={() => setCreating(false)}
          onSubmit={async (v) => {
            try {
              const c = await api.createCollection(v);
              setSelected(c.id);
              setCreating(false);
            } catch (e) {
              app.toast(errorText(e), "error");
            }
          }}
        />
      )}
    </div>
  );
}

export function DocumentsPage() {
  const app = useApp();
  return (
    <div className="flex h-full min-w-0 flex-1 flex-col bg-bg">
      <header className="drag flex h-[52px] shrink-0 items-center gap-3 border-b border-line px-6">
        <h1 className="text-[14px] font-semibold">{t("Documentos")}</h1>
        {app.embedder?.state === "running" && <span className="text-[11.5px] text-muted">{t("Embeddings: {name}", { name: app.embedder.modelName })}</span>}
        {app.embedder?.state === "starting" && (
          <span className="flex items-center gap-1 text-[11.5px] text-muted">
            <Loader2 className="size-3 animate-spin" /> {t("Carregando {name}…", { name: app.embedder.modelName })}
          </span>
        )}
      </header>
      {app.runtime?.state === "missing" ? (
        <EmptyState icon={<BookOpen className="size-10" />} title={t("Instale o llamafile primeiro")} text={t("Os embeddings rodam no llamafile.")} action={<Button onClick={() => app.setPage("settings")}>{t("Configurações")}</Button>} />
      ) : (
        <ModelGate category="embedding" models={app.embedModels} icon={<BookOpen className="size-6" />}>
          <Collections />
        </ModelGate>
      )}
    </div>
  );
}
