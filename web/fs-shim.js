"use strict";

// Go js/wasm の globalThis.fs をインメモリ仮想FSで置き換える。
// Goの fsCall はコールバックを待って goroutine を止めるため、コールバックは同期で呼ぶ。
// 非同期にすると、JSからGo関数を同期的に呼んだ時にイベントループが回らずデッドロックする。

function normalizePath(input) {
  if (input === undefined || input === null || input === "") return "/";
  let value = String(input).replace(/\\/g, "/");
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
  return "/" + parts.join("/");
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

function createVirtualFs() {
  const root = { type: "dir", children: new Map() };
  const handles = new Map();
  let nextFd = 3;

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

  function lookup(path) {
    const normalized = normalizePath(path);
    if (normalized === "/") return root;
    let node = root;
    for (const segment of normalized.slice(1).split("/")) {
      if (!node || node.type !== "dir") return null;
      node = node.children.get(segment);
    }
    return node || null;
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
    return node;
  }

  function makeStats(node, isDir) {
    const data = node.type === "file" ? node.data : new Uint8Array(0);
    return {
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
      parent.children.set(basePath(path), { type: "file", data: bytes });
    },
    mountText(path, text) {
      fs.mountFile(path, new TextEncoder().encode(text));
    },
    // テスト・アプリ側から仮想FSの内容を読むための拡張。
    readFile(path) {
      const node = lookup(path);
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
    exists(path) {
      return lookup(path) !== null;
    },
    _open(path, flags) {
      const normalized = normalizePath(path);
      let node = lookup(normalized);
      const create = (flags & constants.O_CREAT) !== 0;
      if (!node) {
        if (!create) throw makeError("ENOENT");
        ensureDir(parentPath(normalized));
        node = { type: "file", data: new Uint8Array(0) };
        ensureDir(parentPath(normalized)).children.set(basePath(normalized), node);
      }
      if (flags & constants.O_DIRECTORY && node.type !== "dir") throw makeError("ENOTDIR");
      if (node.type === "dir" && flags & (constants.O_WRONLY | constants.O_RDWR)) throw makeError("EISDIR");
      if (flags & constants.O_TRUNC && node.type === "file") node.data = new Uint8Array(0);
      const fd = nextFd++;
      handles.set(fd, { path: normalized, pos: 0, node });
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
      if (!node) throw makeError("ENOENT");
      return makeStats(node, node.type === "dir");
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
      if (!node) throw makeError("ENOENT");
      if (node.type !== "dir") throw makeError("ENOTDIR");
      return Array.from(node.children.keys());
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
        const start = position === null || position === undefined ? handle.pos : Number(position);
        const end = start + length;
        if (handle.node.data.length < end) {
          const grown = new Uint8Array(end);
          grown.set(handle.node.data);
          handle.node.data = grown;
        }
        handle.node.data.set(buffer.subarray(offset, offset + length), start);
        if (position === null || position === undefined) handle.pos = end;
        callback(null, length);
      } catch (error) {
        callback(error);
      }
    },
    _mkdir(path) {
      ensureDir(path);
    },
    mkdir(path, mode, callback) {
      call("_mkdir", path, callback);
    },
    _unlink(path) {
      const parent = lookup(parentPath(path));
      if (!parent) throw makeError("ENOENT");
      if (!parent.children.delete(basePath(path))) throw makeError("ENOENT");
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
    },
    rmdir(path, callback) {
      call("_rmdir", path, callback);
    },
    _rename(from, to) {
      const node = lookup(from);
      if (!node) throw makeError("ENOENT");
      lookup(parentPath(from)).children.delete(basePath(from));
      ensureDir(parentPath(to));
      lookup(parentPath(to)).children.set(basePath(to), node);
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
    },
    truncate(path, length, callback) {
      call("_truncate", path, length, callback);
    },
    ftruncate(fd, length, callback) {
      try {
        const handle = handles.get(fd);
        if (!handle) throw makeError("EBADF");
        const next = new Uint8Array(Number(length));
        next.set(handle.node.data.subarray(0, Math.min(handle.node.data.length, next.length)));
        handle.node.data = next;
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
  const fs = createVirtualFs();
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
