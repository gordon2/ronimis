import os, sys, csv, datetime as dt, statistics as st
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from lib import *
from deconv_lib import sat_profile, deconvolve, slot_label, G, L, NS

OUT = os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', 'results')
os.makedirs(OUT, exist_ok=True)
SESS=18; OPEN_H, CLOSE_H = 8.0, 23.5

def occ(a,D):
    k=D//G; n=len(a); R=[0.0]*n; s=0.0
    for i in range(n):
        s+=a[i]
        if i-k>=0: s-=a[i-k]
        R[i]=s
    return R
def clock(i): return ((i*G+180)%(24*60))/60.0

C,nd = sat_profile('T1')
a,Aa,rms = deconvolve(C,L,lam=5.0,iters=40000,verbose=False)
R60,R90,R120 = occ(a,60),occ(a,90),occ(a,120)

with open(os.path.join(OUT,'t1_saturday_profile.csv'),'w',newline='') as f:
    w=csv.writer(f); w.writerow(['time','board_C_median','reconstructed_C','entries_per_10min','entries_per_hour','occupancy_D60','occupancy_D90','occupancy_D120'])
    for i in range(NS):
        w.writerow([slot_label(i), f"{C[i]:.1f}", f"{Aa[i]:.2f}", f"{a[i]:.3f}", f"{a[i]*6:.2f}",
                    f"{R60[i]:.2f}", f"{R90[i]:.2f}", f"{R120[i]:.2f}"])

with open(os.path.join(OUT,'t1_saturday_windows.csv'),'w',newline='') as f:
    w=csv.writer(f); w.writerow(['start','end','occupancy_D90_mean','occupancy_D90_peak','occupancy_D60_mean','board_C_mean'])
    for s0 in range(NS):
        if s0+SESS>=NS: continue
        t0,t1 = clock(s0), clock(s0+SESS)
        if t1<t0: t1+=24
        if t0<OPEN_H-1e-9 or t1>CLOSE_H+1e-9: continue
        idx=[s0+j for j in range(SESS)]
        w.writerow([slot_label(s0), slot_label(s0+SESS),
                    f"{st.mean([R90[i] for i in idx]):.2f}", f"{max(R90[i] for i in idx):.2f}",
                    f"{st.mean([R60[i] for i in idx]):.2f}", f"{st.mean([C[i] for i in idx]):.1f}"])

# по дням недели, все залы
with open(os.path.join(OUT,'weekday_summary.csv'),'w',newline='') as f:
    w=csv.writer(f); w.writerow(['location','weekday','days','mean_peak','median_peak'])
    names=['Mon','Tue','Wed','Thu','Fri','Sat','Sun']
    for loc in ['T1','Mustika','Suur-Paala','Hipodroom']:
        rows=load(loc); byd=defaultdict(list)
        for ts,c in rows: byd[ts.date()].append(c)
        for wd in range(7):
            ds=[d for d in byd if d.weekday()==wd]
            if not ds: continue
            mx=[max(byd[d]) for d in ds]
            w.writerow([loc,names[wd],len(ds),f"{st.mean(mx):.1f}",f"{st.median(mx):.1f}"])

# окно счётчика по залам (ACF)
with open(os.path.join(OUT,'counter_window_acf.csv'),'w',newline='') as f:
    w=csv.writer(f); w.writerow(['location','window_minutes','acf_at_min'])
    for loc in ['T1','Mustika','Suur-Paala','Hipodroom']:
        rows=load(loc); grid=fill_gaps(to_grid(rows)); v=[c for _,c in grid]
        d=[v[i]-v[i-1] for i in range(1,len(v))]
        m=st.mean(d); dc=[x-m for x in d]; var=sum(x*x for x in dc)
        best=min(((lag, sum(dc[i]*dc[i+lag] for i in range(len(dc)-lag))/var) for lag in range(1,200)), key=lambda x:x[1])
        w.writerow([loc, best[0]*STEP, f"{best[1]:+.4f}"])

print(f"rms деконволюции = {rms:.3f}")
for fn in sorted(os.listdir(OUT)):
    p=os.path.join(OUT,fn); print(f"  {fn}  {os.path.getsize(p)} B")
