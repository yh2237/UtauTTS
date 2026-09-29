"use strict";

// Go js/wasm の globalThis.fs をインメモリ仮想FSで置き換える。
// Goの fsCall はコールバックを待って goroutine を止めるため、コールバックは同期で呼ぶ。
// 非同期にすると、JSからGo関数を同期的に呼んだ時にイベントループが回らずデッドロックする。

const normalizeCache = new Map();

function normalizePath(input) {
  if (input === undefined || input === null || input === "") return "/";
  const key = typeof input === "string" ? input : String(input);
  const cached = normalizeCache.get(key);
  if (cached !== undefined) return cached;
  let value = key.replace(/\\/g, "/");
  if (value[0] !== "/") value = "/" + value;
  const parts = [];
  for (const segment of value.split("/")) {
    if (segment === "" || segment === ".") continue;
    if (segment === "..") {
      parts.pop();
      continue;
    }
    parts.push(segment);
  }
  const result = "/" + parts.join("/");
  if (normalizeCache.size < 50000) normalizeCache.set(key, result);
  return result;
}

function parentPath(path) {
  const normalized = normalizePath(path);
  const index = normalized.lastIndexOf("/");
  return index <= 0 ? "/" : normalized.slice(0, index);
}

function basePath(path) {
  const normalized = normalizePath(path);
  return normalized.slice(normalized.lastIndexOf("/") + 1);
}

function createVirtualFs(options) {
  const root = { type: "dir", children: new Map() };
  const handles = new Map();
  const EMPTY_BYTES = new Uint8Array(0);
  let nextFd = 3;
  let nextVersion = 0;

  function changed(node) {
    node.__stats = null;
    node.version = ++nextVersion;
  }

  // リモート裏付け: 指定プレフィックス配下のファイルは、初回アクセス時に
  // 同期XHRで取得してFSへ載せる（Worker内のみ）。音源WAVを合成時にだけ取得する用途。
  const remote = options && options.remote ? options.remote : null;
  const remotePrefix = remote ? normalizePath(remote.prefix || "/voice") : "";
  const remoteBase = remote ? (remote.baseURL || "") : "";
  const remoteFiles = remote ? (remote.files || {}) : {};

  // 事前索引: ファイル/ディレクトリ/子要素を O(1) で引けるようにする。
  const remoteFileSizes = new Map();
  const remoteDirs = new Set();
  const remoteChildMap = new Map();
  if (remote) {
    remoteDirs.add("");
    for (const key of Object.keys(remoteFiles)) {
      remoteFileSizes.set(key, Number(remoteFiles[key]));
      const parts = key.split("/");
      let dir = "";
      for (let i = 0; i < parts.length - 1; ++i) {
        const name = parts[i];
        if (!remoteChildMap.has(dir)) remoteChildMap.set(dir, new Map());
        remoteChildMap.get(dir).set(name, true);
        dir = dir ? dir + "/" + name : name;
        remoteDirs.add(dir);
      }
      if (!remoteChildMap.has(dir)) remoteChildMap.set(dir, new Map());
      remoteChildMap.get(dir).set(parts[parts.length - 1], false);
    }
  }

  function remoteRelative(path) {
    if (!remote) return null;
    const normalized = normalizePath(path);
    if (normalized === remotePrefix) return "";
    if (!normalized.startsWith(remotePrefix + "/")) return null;
    return normalized.slice(remotePrefix.length + 1);
  }

  function remoteFileSize(rel) {
    return rel !== null && remoteFileSizes.has(rel) ? remoteFileSizes.get(rel) : -1;
  }

  function remoteIsFile(rel) {
    return rel !== null && remoteFileSizes.has(rel);
  }

  function remoteChildren(rel) {
    return rel !== null ? remoteChildMap.get(rel) : undefined;
  }

  const remoteChildrenArrayCache = new Map();
  function remoteChildrenArray(rel) {
    let arr = remoteChildrenArrayCache.get(rel);
    if (arr === undefined) {
      const map = remoteChildMap.get(rel);
      arr = map ? Array.from(map.keys()) : null;
      remoteChildrenArrayCache.set(rel, arr);
    }
    return arr;
  }

  function remoteIsDir(rel) {
    return rel !== null && remoteDirs.has(rel);
  }

  function remoteFetch(rel) {
    if (!remoteBase || rel === null || !remoteIsFile(rel)) return false;
    try {
      const url = remoteBase + rel.split("/").map(encodeURIComponent).join("/");
      const xhr = new XMLHttpRequest();
      xhr.open("GET", url, false);
      xhr.responseType = "arraybuffer";
      xhr.send(null);
      if (xhr.status < 200 || xhr.status >= 300) {
        console.warn("fs-shim remoteFetch HTTP " + xhr.status + " for " + url);
        return false;
      }
      fs.mountFile(remotePrefix + "/" + rel, new Uint8Array(xhr.response));
      return true;
    } catch (error) {
      console.warn("fs-shim remoteFetch failed for " + rel + ": " + error);
      return false;
    }
  }

  function remoteDirStats(size) {
    return {
      dev: 0, ino: 0, mode: 0o40755, nlink: 1, uid: 0, gid: 0, rdev: 0,
      size: 0, blksize: 4096, blocks: 0,
      atimeMs: 0, mtimeMs: 0, ctimeMs: 0, birthtimeMs: 0,
      isDirectory: () => true, isFile: () => false, isSymbolicLink: () => false,
    };
  }

  function remoteFileStats(size) {
    return {
      dev: 0, ino: 0, mode: 0o100644, nlink: 1, uid: 0, gid: 0, rdev: 0,
      size: size, blksize: 4096, blocks: Math.ceil(size / 512),
      atimeMs: 0, mtimeMs: 0, ctimeMs: 0, birthtimeMs: 0,
      isDirectory: () => false, isFile: () => true, isSymbolicLink: () => false,
    };
  }

  const constants = {
    O_WRONLY: 1,
    O_RDWR: 2,
    O_CREAT: 64,
    O_TRUNC: 512,
    O_APPEND: 1024,
    O_EXCL: 128,
    O_DIRECTORY: 65536,
  };

  function makeError(code) {
    const error = new Error(code);
    error.code = code;
    return error;
  }

  const lookupCache = new Map();

  function invalidateLookup() {
    lookupCache.clear();
  }

  function lookup(path) {
    const normalized = normalizePath(path);
    if (normalized === "/") return root;
    if (lookupCache.has(normalized)) return lookupCache.get(normalized);
    let node = root;
    for (const segment of normalized.slice(1).split("/")) {
      if (!node || node.type !== "dir") {
        lookupCache.set(normalized, null);
        return null;
      }
      node = node.children.get(segment);
    }
    const result = node || null;
    lookupCache.set(normalized, result);
    return result;
  }

  function ensureDir(path) {
    const normalized = normalizePath(path);
    if (normalized === "/") return root;
    let node = root;
    for (const segment of normalized.slice(1).split("/")) {
      let child = node.children.get(segment);
      if (!child) {
        child = { type: "dir", children: new Map() };
        node.children.set(segment, child);
      }
      if (child.type !== "dir") throw makeError("ENOTDIR");
      node = child;
    }
    invalidateLookup();
    return node;
  }

  function makeStats(node, isDir) {
    if (node.__stats) return node.__stats;
    const data = node.type === "file" ? node.data : EMPTY_BYTES;
    const stats = {
      dev: 0,
      ino: 0,
      mode: isDir ? 0o40755 : 0o100644,
      nlink: 1,
      uid: 0,
      gid: 0,
      rdev: 0,
      size: data.length,
      blksize: 4096,
      blocks: Math.ceil(data.length / 512),
      atimeMs: 0,
      mtimeMs: 0,
      ctimeMs: 0,
      birthtimeMs: 0,
      isDirectory: () => isDir,
      isFile: () => !isDir,
      isSymbolicLink: () => false,
    };
    node.__stats = stats;
    return stats;
  }

  // 同期コールバックで応答する。例外はerrno相当のErrorで返す。
  function reply(callback, error, value) {
    if (error) callback(error, value);
    else callback(null, value);
  }

  function call(method) {
    const args = Array.prototype.slice.call(arguments, 1);
    const callback = args.pop();
    try {
      const value = fs[method].apply(fs, args);
      if (value === undefined) callback(null);
      else callback(null, value);
    } catch (error) {
      callback(error);
    }
  }

  const fs = {
    constants,
    // テスト・アプリ側から仮想FSへファイルを置くための拡張。
    mountFile(path, data) {
      const bytes = data instanceof Uint8Array ? data : new Uint8Array(data);
      const parent = ensureDir(parentPath(path));
      const node = { type: "file", data: bytes };
      changed(node);
      parent.children.set(basePath(path), node);
      invalidateLookup();
    },
    mountText(path, text) {
      fs.mountFile(path, new TextEncoder().encode(text));
    },
    // テスト・アプリ側から仮想FSの内容を読むための拡張。
    readFile(path) {
      let node = lookup(path);
      if (!node) {
        const rel = remoteRelative(path);
        if (rel !== null && remoteIsFile(rel) && remoteFetch(rel)) node = lookup(path);
      }
      if (!node || node.type !== "file") return null;
      return node.data;
    },
    // 仮想FS内の全ファイルパスを返す（Workerのミラー同期用）。
    listFiles() {
      const result = [];
      const walk = (node, prefix) => {
        for (const [name, child] of node.children) {
          const childPath = prefix + "/" + name;
          if (child.type === "dir") walk(child, childPath);
          else result.push(childPath);
        }
      };
      walk(root, "");
      return result;
    },
    fileVersions() {
      return new Map(fs.listFiles().map(path => [path, lookup(path).version]));
    },
    exists(path) {
      if (lookup(path) !== null) return true;
      const rel = remoteRelative(path);
      return rel !== null && (remoteIsFile(rel) || remoteIsDir(rel));
    },
    _open(path, flags) {
      const normalized = normalizePath(path);
      let node = lookup(normalized);
      const create = (flags & constants.O_CREAT) !== 0;
      const rel = remoteRelative(normalized);
      if (create && (flags & constants.O_EXCL)
          && (node || remoteIsFile(rel) || remoteIsDir(rel))) throw makeError("EEXIST");
      if (!node && (!(flags & constants.O_TRUNC) || !create)) {
        if (rel !== null) {
          if (remoteIsFile(rel) && remoteFetch(rel)) {
            node = lookup(normalized);
          } else if (remoteIsDir(rel)) {
            ensureDir(normalized);
            node = lookup(normalized);
          }
        }
      }
      if (!node) {
        if (!create) throw makeError("ENOENT");
        ensureDir(parentPath(normalized));
        node = { type: "file", data: new Uint8Array(0) };
        changed(node);
        ensureDir(parentPath(normalized)).children.set(basePath(normalized), node);
        invalidateLookup();
      }
      if (flags & constants.O_DIRECTORY && node.type !== "dir") throw makeError("ENOTDIR");
      if (node.type === "dir" && flags & (constants.O_WRONLY | constants.O_RDWR)) throw makeError("EISDIR");
      if (flags & constants.O_TRUNC && node.type === "file") { node.data = new Uint8Array(0); changed(node); }
      const fd = nextFd++;
      handles.set(fd, { path: normalized, pos: 0, node, flags });
      return fd;
    },
    open(path, flags, mode, callback) {
      call("_open", path, flags, callback);
    },
    close(fd, callback) {
      handles.delete(fd);
      reply(callback, null);
    },
    _stat(path) {
      const node = lookup(path);
      if (node) return makeStats(node, node.type === "dir");
      const rel = remoteRelative(path);
      if (rel !== null) {
        if (remoteIsFile(rel)) return remoteFileStats(remoteFileSize(rel));
        if (remoteIsDir(rel)) return remoteDirStats();
      }
      throw makeError("ENOENT");
    },
    stat(path, callback) {
      call("_stat", path, callback);
    },
    lstat(path, callback) {
      call("_stat", path, callback);
    },
    _fstat(fd) {
      const handle = handles.get(fd);
      if (!handle) throw makeError("EBADF");
      return makeStats(handle.node, handle.node.type === "dir");
    },
    fstat(fd, callback) {
      call("_fstat", fd, callback);
    },
    _readdir(path) {
      const node = lookup(path);
      if (node && node.type !== "dir") throw makeError("ENOTDIR");
      const rel = remoteRelative(path);
      const remoteDir = rel !== null && remoteIsDir(rel);
      if (!node && !remoteDir) throw makeError("ENOENT");
      const names = new Set(node ? node.children.keys() : []);
      const children = remoteDir ? remoteChildrenArray(rel) : null;
      if (children) {
        for (const name of children) names.add(name);
      }
      return Array.from(names);
    },
    readdir(path, callback) {
      call("_readdir", path, callback);
    },
    read(fd, buffer, offset, length, position, callback) {
      try {
        const handle = handles.get(fd);
        if (!handle) throw makeError("EBADF");
        if (handle.node.type !== "file") throw makeError("EISDIR");
        const start = position === null || position === undefined ? handle.pos : Number(position);
        const available = Math.max(0, handle.node.data.length - start);
        const count = Math.min(length, available);
        buffer.set(handle.node.data.subarray(start, start + count), offset);
        if (position === null || position === undefined) handle.pos = start + count;
        callback(null, count);
      } catch (error) {
        callback(error);
      }
    },
    write(fd, buffer, offset, length, position, callback) {
      try {
        const handle = handles.get(fd);
        if (!handle) throw makeError("EBADF");
        if (handle.node.type !== "file") throw makeError("EISDIR");
        const start = (handle.flags & constants.O_APPEND) ? handle.node.data.length
          : position === null || position === undefined ? handle.pos : Number(position);
        const end = start + length;
        if (handle.node.data.length < end) {
          const grown = new Uint8Array(end);
          grown.set(handle.node.data);
          handle.node.data = grown;
        }
        handle.node.data.set(buffer.subarray(offset, offset + length), start);
        changed(handle.node);
        if (position === null || position === undefined) handle.pos = end;
        callback(null, length);
      } catch (error) {
        callback(error);
      }
    },
    _mkdir(path) {
      ensureDir(path);
      invalidateLookup();
    },
    mkdir(path, mode, callback) {
      call("_mkdir", path, callback);
    },
    _unlink(path) {
      const parent = lookup(parentPath(path));
      if (!parent) throw makeError("ENOENT");
      if (!parent.children.delete(basePath(path))) throw makeError("ENOENT");
      invalidateLookup();
    },
    unlink(path, callback) {
      call("_unlink", path, callback);
    },
    _rmdir(path) {
      const normalized = normalizePath(path);
      const parent = lookup(parentPath(normalized));
      const node = lookup(normalized);
      if (!node || node.type !== "dir") throw makeError("ENOENT");
      if (node.children.size !== 0) throw makeError("ENOTEMPTY");
      if (!parent.children.delete(basePath(normalized))) throw makeError("ENOENT");
      invalidateLookup();
    },
    rmdir(path, callback) {
      call("_rmdir", path, callback);
    },
    _rename(from, to) {
      from = normalizePath(from);
      to = normalizePath(to);
      const node = lookup(from);
      if (!node) throw makeError("ENOENT");
      if (from === to) return;
      if (from === "/" || to === "/" || to.startsWith(from + "/")) throw makeError("EINVAL");
      const targetParent = lookup(parentPath(to));
      if (!targetParent) throw makeError("ENOENT");
      if (targetParent.type !== "dir") throw makeError("ENOTDIR");
      const target = lookup(to);
      if (target && target.type !== node.type) throw makeError(target.type === "dir" ? "EISDIR" : "ENOTDIR");
      if (target && target.type === "dir" && target.children.size) throw makeError("ENOTEMPTY");
      lookup(parentPath(from)).children.delete(basePath(from));
      targetParent.children.set(basePath(to), node);
      invalidateLookup();
    },
    rename(from, to, callback) {
      call("_rename", from, to, callback);
    },
    _truncate(path, length) {
      const node = lookup(path);
      if (!node || node.type !== "file") throw makeError("ENOENT");
      const next = new Uint8Array(Number(length));
      next.set(node.data.subarray(0, Math.min(node.data.length, next.length)));
      node.data = next;
      changed(node);
    },
    truncate(path, length, callback) {
      call("_truncate", path, length, callback);
    },
    ftruncate(fd, length, callback) {
      try {
        const handle = handles.get(fd);
        if (!handle) throw makeError("EBADF");
        if (handle.node.type !== "file") throw makeError("EISDIR");
        const next = new Uint8Array(Number(length));
        next.set(handle.node.data.subarray(0, Math.min(handle.node.data.length, next.length)));
        handle.node.data = next;
        changed(handle.node);
        callback(null);
      } catch (error) {
        callback(error);
      }
    },
    fsync(fd, callback) {
      callback(null);
    },
    chmod(path, mode, callback) {
      callback(null);
    },
    fchmod(fd, mode, callback) {
      callback(null);
    },
    chown(path, uid, gid, callback) {
      callback(null);
    },
    fchown(fd, uid, gid, callback) {
      callback(null);
    },
    lchown(path, uid, gid, callback) {
      callback(null);
    },
    utimes(path, atime, mtime, callback) {
      callback(null);
    },
    readlink(path, callback) {
      callback(makeError("EINVAL"));
    },
    link(path, link, callback) {
      callback(makeError("EPERM"));
    },
    symlink(path, link, callback) {
      callback(makeError("EPERM"));
    },
  };

  return fs;
}

// Goのwasm_execが期待するグローバルを、Go起動前に仮想FSへ差し替える。
function installVirtualFs(options) {
  const cwd = (options && options.cwd) || "/";
  const fs = createVirtualFs(options);
  globalThis.fs = fs;
  globalThis.path = {
    resolve: function () {
      return normalizePath(Array.prototype.join.call(arguments, "/"));
    },
  };
  globalThis.process = globalThis.process || {};
  globalThis.process.cwd = function () {
    return cwd;
  };
  globalThis.process.chdir = function () {};
  return fs;
}

if (typeof module === "object" && module.exports) {
  module.exports = { createVirtualFs, installVirtualFs, normalizePath };
}
