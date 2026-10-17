<p align="center">
  <img src="docs/icon.png" width="96" alt="Llamafile UI" />
</p>

<h1 align="center">Llamafile UI</h1>

<p align="center">
  Chat, imagens, voz e documentos com IA — rodando 100% no seu computador.<br/>
  Uma interface desktop para o <a href="https://github.com/mozilla-ai/llamafile">llamafile</a> da Mozilla AI,
  que mostra quais modelos rodam melhor na <b>sua</b> máquina.
</p>

<p align="center">
  <a href="https://github.com/devalexandre/llamafile-ui/releases/latest"><img src="https://img.shields.io/github/v/release/devalexandre/llamafile-ui?label=download&color=bd93f9" alt="Download" /></a>
  <img src="https://img.shields.io/badge/Linux%20%C2%B7%20macOS%20%C2%B7%20Windows-282a36" alt="Plataformas" />
  <img src="https://img.shields.io/badge/Go%20%2B%20Wails%20%2B%20React-61afef" alt="Stack" />
</p>

<p align="center">
  <img src="docs/screenshots/chat.png" width="860" alt="Chat com documentos e fontes citadas" />
</p>

## Por que usar

- **Privado de verdade:** conversas, documentos, áudios e imagens nunca saem do seu computador.
- **Sem configuração:** o app baixa o llamafile e os modelos para você e escolhe o que cabe no seu hardware.
- **Tudo em um lugar:** chat, geração de imagens, transcrição de voz e perguntas sobre seus documentos.
- **Feito para modelos pequenos e rápidos:** um catálogo curado de modelos que rodam bem em notebooks e desktops comuns.

## Recursos

### 💬 Chat
Respostas em streaming com markdown e código destacado, raciocínio ("thinking") recolhível,
imagens em modelos de visão (anexar ou colar), parar, regenerar, editar e reenviar, e métricas
de tokens/s. O histórico fica salvo e é **agrupado por data** (Hoje, Ontem, 7 dias…), com fixar,
renomear, buscar no conteúdo, exportar para Markdown/JSON e **títulos gerados automaticamente**.

### 🧭 Descobrir modelos — o que roda bem na sua máquina
O app detecta CPU, RAM, GPU/VRAM (NVIDIA, AMD e Apple Silicon) e, para cada modelo, calcula a
memória necessária, onde ele roda (GPU, GPU+CPU ou CPU), a velocidade estimada e uma nota por
categoria. Em cada categoria ele destaca a **melhor escolha para o seu computador** e explica o porquê.

<p align="center"><img src="docs/screenshots/discover.png" width="760" alt="Descobrir modelos" /></p>

### 🎨 Imagens
Geração local com SD-Turbo, SDXL-Turbo, Stable Diffusion 1.5, Z-Image Turbo e FLUX.1 schnell.
Formatos e tamanhos, passos/CFG/seed, **decodificação rápida (TAESD)**, progresso com tempo
estimado e uma galeria para reutilizar, salvar ou "perguntar no chat". O botão **Melhorar prompt**
usa o modelo de chat local para traduzir e detalhar sua descrição em inglês.

<p align="center"><img src="docs/screenshots/images.png" width="760" alt="Geração de imagens" /></p>

### 🎙️ Voz
**Ditado no chat** com o botão de microfone e uma página de **Transcrição**: grave ou arraste
arquivos WAV, MP3, OGG ou FLAC, escolha o idioma, copie, exporte `.txt` ou legendas `.srt`, ou
resuma no chat. Usa os modelos Whisper; o áudio gravado é apagado depois da transcrição.

<p align="center"><img src="docs/screenshots/voice.png" width="760" alt="Transcrição de voz" /></p>

### 📚 Documentos (RAG)
Crie coleções com PDF, Word (.docx), Markdown, texto, HTML ou código. Os arquivos são indexados
localmente com um modelo de embeddings (Qwen3 Embedding, BGE-M3 ou Nomic). No chat, escolha uma
coleção: a resposta usa os trechos mais relevantes e mostra as **fontes citadas** ([1], [2]…),
que você pode abrir para conferir.

<p align="center"><img src="docs/screenshots/docs.png" width="760" alt="Documentos" /></p>

### ⚙️ Configurações e mais
- Porta e endereço do servidor, aceleração (GPU), tamanho do contexto, threads e iniciar com o app.
- Instruções do sistema, temperatura e títulos automáticos.
- Tema **Nocturne**, próprio do app (inspirado no Dracula e no One Dark), nos modos claro e escuro.
- O servidor expõe uma **API compatível com OpenAI** em `http://127.0.0.1:8686/v1`, que outras
  ferramentas podem usar enquanto o app está aberto.

<p align="center"><img src="docs/screenshots/discover-light.png" width="760" alt="Tema claro" /></p>

## Instalação

Baixe a versão mais recente na página de [Releases](https://github.com/devalexandre/llamafile-ui/releases/latest):

| Sistema | Arquivo | Como abrir |
|---|---|---|
| **Windows** (x64) | `llamafile-ui-<versão>-windows-amd64-setup.exe` (instalador) ou `…-windows-amd64.exe` (portátil) | Se o SmartScreen avisar, clique em **Mais informações → Executar assim mesmo**. |
| **macOS** (Intel e Apple Silicon) | `llamafile-ui-<versão>-macos-universal.zip` | Descompacte e mova para Aplicativos. Na primeira vez, clique com o **botão direito → Abrir**. |
| **Linux** (x64) | `llamafile-ui-<versão>-linux-amd64.tar.gz` | `tar xzf llamafile-ui-*.tar.gz && ./llamafile-ui` |

No Linux é preciso ter **WebKitGTK 4.1** e **GTK 3**, que já vêm na maioria das distribuições:

```bash
sudo apt install libwebkit2gtk-4.1-0 libgtk-3-0     # Debian/Ubuntu
sudo pacman -S webkit2gtk-4.1 gtk3                  # Arch
sudo dnf install webkit2gtk4.1 gtk3                 # Fedora
```

Os binários não são assinados; por isso os avisos do macOS e do Windows na primeira execução.

## Primeiros passos

1. **Instale o llamafile.** Na primeira abertura, o app vai direto para Configurações e oferece:
   - **Versão completa** (~350 MB): usa a GPU, recomendada se você tem placa de vídeo.
   - **Versão leve** (~42 MB): só CPU.
   - **Já tenho o arquivo…**: aponta para um llamafile que você já tem.
2. **Escolha um modelo** em *Descobrir modelos*. Comece pela "Melhor escolha para o seu computador".
3. **Converse.** O modelo carrega sozinho na primeira mensagem.
4. Para **imagens** e **voz**, as páginas oferecem baixar o `diffusionfile` / `whisperfile` (os
   executáveis da mesma release oficial) e um modelo recomendado.
5. Para **documentos**, baixe um modelo de embeddings (são pequenos), crie uma coleção, arraste os
   arquivos e clique em **Conversar**.

## Modelos do catálogo

| Categoria | Modelos |
|---|---|
| Chat | Qwen3 (1.7B, 4B, 8B), Gemma 3 (1B, 4B), Llama 3.2 (1B, 3B), Phi-4 mini, Granite 3.3 2B, SmolLM2, Mistral 7B, gpt-oss 20B |
| Código | Qwen2.5 Coder (1.5B, 3B, 7B), Qwen3 |
| Raciocínio | DeepSeek R1 Distill (1.5B, 7B), Qwen3, Phi-4 mini, gpt-oss 20B |
| Visão | Gemma 3 4B, Qwen2.5 VL 3B, SmolVLM2 2.2B |
| Imagens | SD-Turbo, SDXL-Turbo, Stable Diffusion 1.5, Z-Image Turbo, FLUX.1 schnell |
| Voz | Whisper Tiny, Base, Small, Medium e Large v3 Turbo |
| Embeddings | Qwen3 Embedding 0.6B, BGE-M3, Nomic Embed v1.5 |

Arquivos `.gguf` colocados manualmente na pasta de modelos também aparecem em *Meus modelos* e
podem ser usados no chat.

## Como a recomendação funciona

O método é inspirado no [llmfit](https://github.com/AlexsJones/llmfit), mas adaptado para
GGUF/llamafile e sem dependências externas (`internal/catalog/fit.go`):

| Etapa | Como é calculado |
|---|---|
| Memória necessária | pesos × 1,05 + projetor de visão + componentes + KV cache (pelo contexto) + overhead |
| Onde roda | GPU se cabe na VRAM; GPU+CPU se cabe em VRAM+RAM; senão CPU (Apple: memória unificada) |
| Encaixe | uso ≤ 60% roda perfeitamente · ≤ 85% roda bem · ≤ 98% no limite · acima disso não cabe |
| Velocidade (texto) | largura de banda da memória ÷ bytes lidos por token × eficiência (MoE conta só os parâmetros ativos) |
| Velocidade (imagem/voz) | segundos por imagem e fator de tempo real, calibrados em execuções reais |
| Nota final | média ponderada de qualidade, velocidade, encaixe e contexto, com pesos por categoria |

Se o llamafile instalado for a versão leve, as recomendações assumem CPU e o app sugere a versão
completa para aproveitar a GPU.

## Privacidade

O app só acessa a internet quando você pede: para baixar executáveis (GitHub da Mozilla AI) e
modelos (Hugging Face). Os dados ficam em:

- **Linux:** `~/.local/share/LlamafileUI`
- **macOS:** `~/Library/Application Support/LlamafileUI`
- **Windows:** `%LOCALAPPDATA%\LlamafileUI`

Nessa pasta estão os modelos, as conversas e os documentos indexados (`chats.db`, SQLite), as
imagens geradas e os executáveis. Configurações → *Dados do aplicativo* abre essa pasta.

## Solução de problemas

| Problema | O que fazer |
|---|---|
| **"A porta 8686 já está em uso"** | Mude a porta em Configurações → Rede e clique em *Reiniciar para aplicar*. |
| **Respostas lentas** | Veja se está usando a versão leve do llamafile (aviso em Descobrir) ou escolha um modelo menor. |
| **Imagens não ficam como descrito** | Use prompts em inglês ou o botão **Melhorar prompt**. |
| **Imagem/voz lentas** | Nesta release, `diffusionfile` e `whisperfile` rodam na CPU (Metal no Mac). Prefira SD-Turbo e Whisper Base/Small. |
| **Janela em branco no Linux com NVIDIA** | O app já define `WEBKIT_DISABLE_DMABUF_RENDERER=1`. Se persistir, rode `WEBKIT_DISABLE_COMPOSITING_MODE=1 ./llamafile-ui`. |
| **llamafile não inicia no Linux** | O app executa o llamafile via `sh`, para não esbarrar no `binfmt_misc` (ex.: Wine). Veja o log em Configurações → Diagnóstico. |
| **Microfone não funciona** | Confira a permissão de microfone do sistema (macOS: Ajustes → Privacidade → Microfone). |

## Desenvolvimento

Requisitos: Go 1.25+, Node 20+, [Wails CLI v2](https://wails.io/docs/gettingstarted/installation)
e, no Linux, `webkit2gtk-4.1` e `gtk3` (pacotes de desenvolvimento). A gravação de áudio usa
[miniaudio](https://github.com/gen2brain/malgo) via cgo, então é preciso um compilador C (gcc ou clang).

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0

wails dev -tags webkit2_41      # Linux (WebKitGTK 4.1)
wails dev                       # macOS / Windows

wails build -tags webkit2_41    # gera build/bin/llamafile-ui
```

### Testes

```bash
go test ./...

# Integração com os executáveis reais (precisam estar instalados pelo app):
go test -tags integration ./internal/chat ./internal/rag ./internal/imagegen -v
VOICE_SAMPLE=fala.wav go test -tags integration ./internal/voice -v
```

### Atualizar o catálogo de modelos

Edite `tools/catalog/models.json` (metadados, notas de qualidade e componentes) e rode:

```bash
go run ./tools/catalog
```

O gerador consulta a API do Hugging Face para preencher URLs e tamanhos reais e grava
`internal/catalog/catalog.json`, que é embutido no binário. Componentes compartilhados (como o
VAE do FLUX) são baixados uma única vez.

### Estrutura

```
main.go, app*.go           Wails: janela e métodos expostos ao frontend (um arquivo por área)
internal/
  catalog/                 catálogo embutido + fit engine (recomendação)
  hardware/                detecção de CPU/RAM/GPU por sistema operacional
  tools/                   llamafile, whisperfile e diffusionfile: release, download, execução via sh
  runtime/                 servidores llamafile (chat e embeddings): iniciar, health-check, parar
  downloads/               downloads retomáveis
  chat/                    SQLite das conversas + streaming da API OpenAI + RAG no prompt
  rag/                     extração de texto, chunks, embeddings e busca por similaridade
  imagegen/                geração com diffusionfile, progresso e galeria
  voice/                   gravação do microfone (miniaudio) e transcrição com whisperfile
  settings/, paths/        preferências e diretórios do app
frontend/src/
  pages/                   Chat, Imagens, Transcrição, Documentos, Descobrir, Meus modelos, Configurações
  components/              Sidebar, cartões de modelo, controles no estilo macOS
tools/catalog/             gerador do catálogo
```

### Releases

A cada push ou merge em `main`/`master`, o workflow [`release.yml`](.github/workflows/release.yml)
roda os testes, compila para Linux (amd64), Windows (amd64, `.exe` + instalador NSIS) e macOS
(universal) e publica uma release `v0.1.<número da execução>` com os arquivos e checksums SHA-256.

## Próximos passos

- Benchmark real de tok/s na sua máquina para refinar as estimativas.
- GPU no `whisperfile`/`diffusionfile` quando a release passar a incluir.
- Leitura das respostas em voz alta (TTS).

## Créditos

Construído sobre projetos incríveis: [llamafile](https://github.com/mozilla-ai/llamafile)
(Mozilla AI), [llama.cpp](https://github.com/ggml-org/llama.cpp),
[whisper.cpp](https://github.com/ggml-org/whisper.cpp),
[stable-diffusion.cpp](https://github.com/leejet/stable-diffusion.cpp), [Wails](https://wails.io)
e [miniaudio](https://miniaud.io). Recomendações inspiradas no [llmfit](https://github.com/AlexsJones/llmfit);
tema inspirado no [Dracula](https://draculatheme.com) e no [One Dark](https://github.com/atom/atom/tree/master/packages/one-dark-ui).
Os modelos pertencem aos seus autores e seguem as respectivas licenças.
