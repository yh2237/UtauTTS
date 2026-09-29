import json
import os
import sys
import zipfile


def decode_name(info):
    if info.flag_bits & 0x800:
        return info.filename
    try:
        return info.filename.encode("cp437").decode("cp932")
    except Exception:
        return info.filename


def main():
    zip_path, out_dir = sys.argv[1], sys.argv[2]
    os.makedirs(out_dir, exist_ok=True)
    entries = []
    with zipfile.ZipFile(zip_path) as zf:
        for info in zf.infolist():
            if info.is_dir():
                continue
            entries.append((info, decode_name(info).replace("\\", "/")))
        # oto.ini を含む最も浅いディレクトリを音源ルートにする。
        bank_rel = None
        best_depth = None
        for info, name in entries:
            if os.path.basename(name).lower() == "oto.ini":
                depth = name.count("/")
                if best_depth is None or depth < best_depth:
                    best_depth = depth
                    bank_rel = name.rsplit("/", 1)[0] if "/" in name else ""
        if bank_rel is None:
            bank_rel = ""
        bank_name = os.path.basename(bank_rel) if bank_rel else os.path.splitext(os.path.basename(zip_path))[0]
        files = {}
        for info, name in entries:
            if bank_rel:
                if name == bank_rel or not name.startswith(bank_rel + "/"):
                    continue
                rel = name[len(bank_rel) + 1:]
            else:
                rel = name
            if not rel:
                continue
            dest = os.path.join(out_dir, bank_name, *rel.split("/"))
            os.makedirs(os.path.dirname(dest), exist_ok=True)
            with zf.open(info) as src, open(dest, "wb") as dst:
                dst.write(src.read())
            files[bank_name + "/" + rel] = os.path.getsize(dest)
    with open(os.path.join(out_dir, "manifest.json"), "w", encoding="utf-8") as handle:
        json.dump({"files": files}, handle, ensure_ascii=False)
    print("packaged voice bank %s: %d files" % (bank_name, len(files)))


if __name__ == "__main__":
    main()
