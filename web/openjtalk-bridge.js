"use strict";

function createOpenJTalkBridge(Module) {
  const init = Module.cwrap("UtauTTSOpenJTalkInit", "number", ["string"]);
  const runNative = Module.cwrap("UtauTTSOpenJTalkRun", "number", ["string", "number", "number"]);
  const lastError = Module.cwrap("UtauTTSOpenJTalkError", "string", []);
  const capacity = 1 << 18;
  const buffer = Module._malloc(capacity);
  return {
    ready: false,
    init(dictionaryPath) {
      if (!init(dictionaryPath)) {
        throw new Error(lastError() || "failed to load Open JTalk dictionary");
      }
      this.ready = true;
    },
    run(text) {
      try {
        const written = runNative(text, buffer, capacity);
        if (written <= 0) {
          return { ok: false, error: lastError() || "Open JTalk run failed" };
        }
        return { ok: true, tsv: Module.UTF8ToString(buffer, written) };
      } catch (error) {
        return { ok: false, error: String(error) };
      }
    },
  };
}

if (typeof module === "object" && module.exports) {
  module.exports = { createOpenJTalkBridge };
}
