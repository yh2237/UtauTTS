#!/usr/bin/env python3
"""Fit a compact V9 residual model from accepted Intonation Lab .utautts sessions."""
from __future__ import annotations
import argparse,copy,hashlib,importlib.util,json,math,random,sys
from pathlib import Path
from typing import Any
import torch
import torch.nn.functional as F
ROOT=Path(__file__).resolve().parent
EXTRA=("base_pitch_cents","base_prev_cents","base_next_cents","base_delta_prev","base_delta_next","base_second_difference","base_phrase_min","base_phrase_max","base_phrase_range","base_near_render_limit")
def load_trainer():
 s=importlib.util.spec_from_file_location("frame_trainer",ROOT/"train-frame-intonation-tcn.py")
 if not s or not s.loader: raise RuntimeError("cannot load frame trainer")
 m=importlib.util.module_from_spec(s);sys.modules[s.name]=m;s.loader.exec_module(m);return m
def nums(v):
 out=[]
 for x in v if isinstance(v,list) else []:
  try: x=float(x)
  except (TypeError,ValueError): x=0
  out.append(x if math.isfinite(x) else 0)
 return out
def at(v,ms,t):
 if not v or ms<=0:return 0
 p=max(0,t/ms);a=min(len(v)-1,int(p));b=min(len(v)-1,a+1)
 return v[a]*(b-p)+v[b]*(p-a)
def timing(e,m):
 d,p=nums(e.get("mora_durations_ms")),nums(e.get("mora_positions_ms"))
 if len(d)!=len(m):d=nums(e.get("automatic_mora_durations_ms"))
 if len(d)!=len(m):d=[180 if x.get("pause") else 120 for x in m]
 d=[max(1,x) for x in d]
 if len(p)!=len(m):p=nums(e.get("automatic_mora_positions_ms"))
 if len(p)!=len(m):
  p=[];x=0
  for q in d:p.append(x);x+=q
 return p,d
def tokenise(e):
 cache=(e.get("analysis_cache") or {}).get("morae")
 if not isinstance(cache,list) or not cache:raise ValueError("analysis_cache.morae is missing")
 try:from openjtalk_features import analyze
 except Exception as x:raise RuntimeError("Open JTalk features are unavailable") from x
 _,accent=analyze(str(e.get("text","")))
 if len(cache)!=len(accent):raise ValueError("Open JTalk mora count differs")
 out=[]
 for i,(raw,a) in enumerate(zip(cache,accent)):
  pause=bool(raw.get("pause"))
  if pause!=bool(a.get("pause")) or (not pause and str(raw.get("mora",""))!=str(a.get("mora",""))):raise ValueError(f"mora differs at {i}")
  x=dict(raw);x.update(a);out.append(x)
 return out
def phrase_ids(m):
 out=[];q=-1
 for i,x in enumerate(m):
  if x.get("pause"):out.append(-1);continue
  if i==0 or m[i-1].get("pause") or x.get("accent_phrase_start"):q+=1
  out.append(q)
 return out
def rows(trainer,m,base):
 ids=phrase_ids(m);r={}
 for i,q in enumerate(ids):
  if q>=0:
   a,b=r.get(q,(base[i],base[i]));r[q]=(min(a,base[i]),max(b,base[i]))
 out=[]
 for i in range(len(m)):
  prev=base[i-1] if i and ids[i-1]==ids[i] else base[i];nxt=base[i+1] if i+1<len(m) and ids[i+1]==ids[i] else base[i]
  x=dict(trainer.token_features(m,i));x.update({"base_pitch_cents":base[i]/120,"base_prev_cents":prev/120,"base_next_cents":nxt/120,"base_delta_prev":(base[i]-prev)/120,"base_delta_next":(nxt-base[i])/120,"base_second_difference":(nxt-2*base[i]+prev)/120,"base_near_render_limit":abs(base[i])/90})
  if ids[i]>=0:
   a,b=r[ids[i]];x.update({"base_phrase_min":a/120,"base_phrase_max":b/120,"base_phrase_range":(b-a)/120})
  out.append(x)
 return out
def samples(trainer,paths,limit):
 out=[];accepted=0;skipped=[]
 for path in paths:
  for pos,e in enumerate(json.loads(path.read_text(encoding="utf-8")).get("utterances",[])):
   if not isinstance(e,dict) or not e.get("training_accepted"):continue
   accepted+=1;key=f"{path.name}:{e.get('lab_entry_id',pos)}"
   try:
    m=tokenise(e);ms=float(e.get("automatic_frame_ms") or 10);auto=nums(e.get("automatic_frame_pitch"))
    if ms<=0 or len(auto)<2:raise ValueError("automatic frame contour is missing")
    start,duration=timing(e,m);base=[at(auto,ms,s+d/2) for s,d in zip(start,duration)]
    frames,points=nums(e.get("pitch_frames")),nums(e.get("pitch_points"));target=[]
    for i,(s,d) in enumerate(zip(start,duration)):
     if frames:
      a=max(0,math.ceil(s/ms));b=min(len(frames),math.ceil((s+d)/ms));v=sorted(frames[a:b]);z=v[len(v)//2] if v else at(frames,ms,s+d/2)
     else:z=points[i] if i<len(points) else 0
     target.append(max(-limit,min(limit,z)))
    x,ids=rows(trainer,m,base),phrase_ids(m)
    for q in sorted(set(i for i in ids if i>=0)):
     take=[i for i,v in enumerate(ids) if v==q];out.append({"id":key,"x":[x[i] for i in take],"y":[target[i] for i in take]})
   except Exception as x:skipped.append(f"{key}: {x}")
 return out,accepted,skipped
def index_of(records):
 names=set(EXTRA)
 for r in records:
  for x in r["x"]:names.update(x)
 return {x:i for i,x in enumerate(sorted(names))}
def make_batch(records,index,scale):
 n=max(len(r["y"]) for r in records);x=torch.zeros((len(records),n,len(index)));y=torch.zeros((len(records),n));mask=torch.zeros((len(records),n),dtype=torch.bool)
 for i,r in enumerate(records):
  for j,z in enumerate(r["x"]):
   for k,v in z.items():
    if v and k in index:x[i,j,index[k]]=v
  y[i,:len(r["y"])]=torch.tensor(r["y"])/scale;mask[i,:len(r["y"])]=True
 return x,y,mask
@torch.no_grad()
def score(model,records,index,scale,device):
 model.eval();out=[]
 for r in records:
  x,y,m=make_batch([r],index,scale);out+=(model(x.to(device)).cpu()[m]*scale-y[m]*scale).abs().tolist()
 return sum(out)/len(out) if out else 0
def export(model,base,raw,index,args,accepted,records,mae):
 names=[None]*len(index)
 for x,i in index.items():names[i]=x
 residual={"feature_names":names,"input_weights":model.input.weight.detach().cpu().double().tolist(),"input_bias":model.input.bias.detach().cpu().double().tolist(),"layers":[{"dilation":int(d),"weights":l.weight.detach().cpu().double().tolist(),"bias":l.bias.detach().cpu().double().tolist()} for d,l in zip(model.dilations,model.layers)],"output_weight":(model.output.weight.detach().cpu().double().squeeze(0)*args.target_scale).tolist(),"output_bias":float(model.output.bias.detach().cpu())*args.target_scale}
 out=copy.deepcopy(base);out.pop("mora_duration",None);out.pop("english_intonation",None)
 out.update({"id":args.model_id or Path(args.out).stem,"display_name":args.display_name or Path(args.out).stem,"description":args.description or "Manual intonation residual trained in UtauTTS Intonation Lab","version":11,"feature_version":2,"mode":"intonation_frame_manual_residual","outputs":{"frame_pitch":True,"mora_pitch_residual":True},"duration_weights":{},"mora_pitch_residual":residual,"base_model":{"id":str(base.get("id","")),"sha256":hashlib.sha256(raw).hexdigest()},"residual_limits":{"low_cents":-args.maximum_residual,"high_cents":args.maximum_residual,"smoothing_ms":args.smoothing_ms},"metrics":{"records":accepted,"tokens":records,"pitch_mae_cents":mae},"training":{"records":accepted,"tokens":records,"epochs":args.epochs,"learning_rate":args.learning_rate,"seed":args.seed}})
 return out
def main():
 p=argparse.ArgumentParser(description=__doc__);p.add_argument("sessions",nargs="+",type=Path);p.add_argument("--base-model",required=True,type=Path);p.add_argument("--out",required=True,type=Path);p.add_argument("--model-id",default="");p.add_argument("--display-name",default="");p.add_argument("--description",default="");p.add_argument("--hidden",type=int,default=24);p.add_argument("--epochs",type=int,default=500);p.add_argument("--batch-size",type=int,default=8);p.add_argument("--learning-rate",type=float,default=.003);p.add_argument("--target-scale",type=float,default=120);p.add_argument("--maximum-residual",type=float,default=180);p.add_argument("--smoothing-ms",type=float,default=20);p.add_argument("--zero-prior",type=float,default=.002);p.add_argument("--delta-weight",type=float,default=.2);p.add_argument("--validation-fraction",type=float,default=.2);p.add_argument("--min-entries",type=int,default=8);p.add_argument("--seed",type=int,default=17);p.add_argument("--device",choices=("auto","cpu","cuda"),default="auto");args=p.parse_args()
 if min(args.hidden,args.epochs,args.target_scale,args.maximum_residual)<=0:raise ValueError("positive model dimensions required")
 if not 0<=args.validation_fraction<1:raise ValueError("validation fraction must be in [0, 1)")
 trainer=load_trainer();raw=args.base_model.read_bytes();base=json.loads(raw)
 if not base.get("id") or not isinstance(base.get("frame_pitch"),dict):raise ValueError("base model must have id and frame_pitch")
 records,accepted,skipped=samples(trainer,args.sessions,args.maximum_residual)
 for x in skipped:print(f"skip: {x}",file=sys.stderr)
 if accepted<args.min_entries:raise ValueError(f"only {accepted} accepted entries; need at least {args.min_entries}")
 if not records:raise ValueError("no usable sounding phrases")
 index=index_of(records);ids=sorted(set(r["id"] for r in records));random.Random(args.seed).shuffle(ids);n=round(len(ids)*args.validation_fraction) if len(ids)>=5 else 0;validation_ids=set(ids[:n]);validation=[r for r in records if r["id"] in validation_ids];training=[r for r in records if r["id"] not in validation_ids] or records
 device=torch.device("cuda" if args.device=="auto" and torch.cuda.is_available() else args.device);torch.manual_seed(args.seed);model=trainer.FrameIntonationTCN(len(index),args.hidden).to(device);opt=torch.optim.AdamW(model.parameters(),lr=args.learning_rate,weight_decay=1e-4);best=float("inf");state=None;rng=random.Random(args.seed)
 for epoch in range(args.epochs):
  model.train();order=list(training);rng.shuffle(order)
  for i in range(0,len(order),args.batch_size):
   x,y,m=make_batch(order[i:i+args.batch_size],index,args.target_scale);x,y,m=x.to(device),y.to(device),m.to(device);z=model(x);absolute=F.smooth_l1_loss(z[m],y[m]);pairs=m[:,1:]&m[:,:-1];delta=F.smooth_l1_loss((z[:,1:]-z[:,:-1])[pairs],(y[:,1:]-y[:,:-1])[pairs]) if bool(pairs.any()) else z.sum()*0;loss=absolute+args.delta_weight*delta+args.zero_prior*z[m].square().mean();opt.zero_grad();loss.backward();torch.nn.utils.clip_grad_norm_(model.parameters(),1);opt.step()
  now=score(model,validation or training,index,args.target_scale,device)
  if now<best:best,state=now,copy.deepcopy(model.state_dict())
  if (epoch+1)%50==0 or epoch+1==args.epochs:print(f"epoch {epoch+1}/{args.epochs}: mae={now:.2f} cents",flush=True)
 if state:model.load_state_dict(state)
 args.out.parent.mkdir(parents=True,exist_ok=True);args.out.write_text(json.dumps(export(model,base,raw,index,args,accepted,len(records),score(model,validation or training,index,args.target_scale,device)),ensure_ascii=False,indent=2)+"\n",encoding="utf-8");print(f"wrote {args.out} from {accepted} accepted entries and {len(records)} phrases")
if __name__=="__main__":main()
