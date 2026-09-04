import os
import sys, datetime as dt, statistics as st
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from lib import *
from deconv_lib import sat_profile, deconvolve, slot_label, G, L, NS

SESS = 18          # 3ч
OPEN_H, CLOSE_H = 8.0, 23.5

def occ(a, D):
    k=D//G; n=len(a); R=[0.0]*n; s=0.0
    for i in range(n):
        s+=a[i]
        if i-k>=0: s-=a[i-k]
        R[i]=s
    return R

def clock(i):
    return ((i*G + 180) % (24*60))/60.0

C,_ = sat_profile('T1')
a,Aa,rms = deconvolve(C,L,lam=5.0,iters=40000,verbose=False)
R90 = occ(a,90); R60 = occ(a,60)

cands=[]
for s0 in range(NS):
    if s0+SESS >= NS: continue
    t0 = clock(s0); t1 = clock(s0+SESS)
    if t1 < t0: t1 += 24
    if t0 < OPEN_H - 1e-9 or t1 > CLOSE_H + 1e-9: continue
    idx=[s0+j for j in range(SESS)]
    cands.append((slot_label(s0), slot_label(s0+SESS),
                  st.mean([R90[i] for i in idx]), max(R90[i] for i in idx),
                  st.mean([R60[i] for i in idx]), st.mean([C[i] for i in idx])))

print(f"=== T1, СУББОТА, сессия 3ч, окно работы {OPEN_H:.0f}:00–{CLOSE_H:.1f} ===")
print(f"(деконволюция окна счётчика 180 мин, rms={rms:.3f}, реальная занятость при тренировке 90 мин)\n")
print("старт–конец      реальн.занятость   пик    табло C")
print("                   (среднее)              (среднее)")
for r in cands:
    if int(r[0][3:]) % 30: continue
    print(f"{r[0]}–{r[1]}          {r[2]:5.2f}        {r[3]:5.2f}     {r[5]:5.1f}")

print("\n--- ТОП-6 самых тихих ---")
for r in sorted(cands,key=lambda x:x[2])[:6]:
    print(f"  {r[0]}–{r[1]}   занятость ср={r[2]:.2f} пик={r[3]:.2f}   табло={r[5]:.1f}")
print("--- самое людное ---")
for r in sorted(cands,key=lambda x:-x[2])[:3]:
    print(f"  {r[0]}–{r[1]}   занятость ср={r[2]:.2f} пик={r[3]:.2f}   табло={r[5]:.1f}")

# --- разброс по отдельным субботам: сырое табло C в ключевых окнах ---
print("\n=== проверка по каждой субботе отдельно (сырое табло, среднее за окно) ===")
rows = load('T1')
grid = fill_gaps(to_grid(rows))
byday=defaultdict(dict)
for t,c in grid:
    anchor = t - dt.timedelta(hours=3)
    byday[anchor.date()][(anchor.hour*60+anchor.minute)//G] = c
sats=[d for d in sorted(byday) if d.weekday()==5]
wins = [("08:00","11:00"),("09:00","12:00"),("12:30","15:30"),("17:00","20:00"),("20:00","23:00"),("20:30","23:30")]
def s2i(s):
    h,m=map(int,s.split(':')); return ((h*60+m) - 180) % (24*60) // G
print("окно            " + "  ".join(str(d)[5:] for d in sats) + "   медиана")
for w0,w1 in wins:
    i0=s2i(w0)
    vals=[]
    for d in sats:
        seq=byday[d]
        v=[seq.get((i0+j)%NS) for j in range(SESS)]
        v=[x for x in v if x is not None]
        vals.append(st.mean(v) if v else float('nan'))
    print(f"{w0}–{w1}   " + "  ".join(f"{x:5.1f}" for x in vals) + f"   {st.median(vals):5.1f}")
