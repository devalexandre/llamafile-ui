import { useCallback, useEffect, useState } from "react";
import { api, on } from "./api";
import { errorText, useApp } from "./store";
import type { Transcript } from "./types";

export type RecorderState = "idle" | "recording" | "transcribing";

/** Microphone recording (in Go) + whisperfile transcription. */
export function useRecorder() {
  const app = useApp();
  const [state, setState] = useState<RecorderState>("idle");
  const [level, setLevel] = useState(0);
  const [elapsed, setElapsed] = useState(0);
  const [progress, setProgress] = useState(0);

  useEffect(() => {
    const offs = [
      on<{ level: number; elapsed: number }>("voice:level", (d) => {
        setLevel(d.level);
        setElapsed(d.elapsed);
      }),
      on<number>("voice:progress", setProgress),
    ];
    return () => offs.forEach((off) => off());
  }, []);

  const ready = (!!app.tool("whisperfile")?.installed || !!app.tool("whisper-gpu")?.installed) && app.voiceModels.length > 0;

  const start = useCallback(async () => {
    try {
      await api.startRecording();
      setElapsed(0);
      setLevel(0);
      setState("recording");
    } catch (e) {
      app.toast(errorText(e), "error");
    }
  }, [app]);

  const stop = useCallback(
    async (save: boolean): Promise<Transcript | null> => {
      setState("transcribing");
      setProgress(0);
      try {
        return await api.stopRecording(save);
      } catch (e) {
        app.toast(errorText(e), "error");
        return null;
      } finally {
        setState("idle");
      }
    },
    [app],
  );

  const cancel = useCallback(() => {
    if (state === "recording") api.cancelRecording();
    else if (state === "transcribing") api.cancelTranscription();
    setState("idle");
  }, [state]);

  return { state, level, elapsed, progress, ready, start, stop, cancel, setState, setProgress };
}

export function clock(seconds: number) {
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  return `${m}:${String(s).padStart(2, "0")}`;
}
