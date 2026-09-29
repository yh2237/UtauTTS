"use strict";

// Workerへ処理を投げる薄いクライアント。UIはこのクラスだけを見る。
class UtauTTSClient {
  constructor(workerURL) {
    this.seq = 0;
    this.pending = new Map();
    this.onStatus = null;
    this.onPreview = null;
    this.onError = null;
    this.ready = new Promise((resolve, reject) => {
      this.resolveReady = resolve;
      this.rejectReady = reject;
    });
    this.worker = new Worker(workerURL);
    this.worker.onmessage = (event) => this.handle(event.data);
    this.worker.onerror = (event) => this.fail(event.message || "worker error");
  }

  handle(data) {
    if (data.id && this.pending.has(data.id)) {
      const entry = this.pending.get(data.id);
      this.pending.delete(data.id);
      if (data.type === "error") entry.reject(new Error(data.message));
      else entry.resolve(data);
      return;
    }
    switch (data.type) {
      case "status":
        if (this.onStatus) this.onStatus(data.text);
        break;
      case "preview":
        if (this.onPreview) this.onPreview(data.preview);
        break;
      case "ready":
        this.resolveReady();
        break;
      case "error":
        this.fail(data.message);
        break;
      default:
        break;
    }
  }

  fail(message) {
    if (this.rejectReady) {
      this.rejectReady(new Error(message));
      this.rejectReady = null;
    }
    if (this.onError) this.onError(message);
  }

  request(type, payload, transfer) {
    const id = ++this.seq;
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      this.worker.postMessage(Object.assign({ type, id }, payload), transfer || []);
    });
  }

  async mountVoicebank(fileList) {
    await this.ready;
    const entries = Array.from(fileList).map((file) => ({
      path: file.webkitRelativePath || file.name,
      file,
    }));
    return this.request("mountVoicebank", { entries });
  }

  async synthesize(text, strength) {
    await this.ready;
    return this.request("synthesize", { text, strength });
  }
}

if (typeof module === "object" && module.exports) {
  module.exports = { UtauTTSClient };
}
globalThis.UtauTTSClient = UtauTTSClient;
