"""WORLDのF0教師を並列抽出し、学習用キャッシュへ保存する。"""
import argparse
from concurrent.futures import ThreadPoolExecutor, as_completed
import importlib.util
import json
from pathlib import Path
import uuid
import numpy as np

def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--dataset',type=Path,required=True); p.add_argument('--cache',type=Path,required=True)
    p.add_argument('--world-engine',required=True); p.add_argument('--workers',type=int,default=4)
    args=p.parse_args()
    if args.workers<1: p.error('workers must be positive')
    spec=importlib.util.spec_from_file_location('trainer',Path(__file__).with_name('train-frame-intonation-tcn.py'))
    trainer=importlib.util.module_from_spec(spec); spec.loader.exec_module(trainer)
    world=trainer.load_worldline(args.world_engine)
    rows=[json.loads(line) for line in args.dataset.read_text(encoding='utf-8').splitlines() if line.strip()]
    args.cache.mkdir(parents=True,exist_ok=True)
    def extract(row):
        path=trainer._f0_cache_path(args.cache,row,10,'utautts_world_harvest')
        if path.exists(): return
        _,f0,_=trainer.extract_record_f0(row,dataset_path=args.dataset,worldline=world)
        temporary=path.with_name(path.name+'.'+uuid.uuid4().hex+'.tmp')
        with temporary.open('wb') as stream: np.save(stream,f0)
        temporary.replace(path)
    with ThreadPoolExecutor(max_workers=args.workers) as executor:
        futures=[executor.submit(extract,row) for row in rows]
        for count,future in enumerate(as_completed(futures),1):
            future.result()
            if count%25==0 or count==len(rows): print(f'F0 cache {count}/{len(rows)}',flush=True)

if __name__=='__main__': main()
