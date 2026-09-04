import os
import sys, datetime as dt, statistics as st
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from lib import *
WIN=180; G=10; L=WIN//G; NS=24*60//G
def sat_profile(loc, weekday=5, pat="gym-stats-202608*.csv", agg='median'):
    rows = load(loc, pat); acc = defaultdict(list)
    for ts,c in rows:
        anchor = ts - dt.timedelta(hours=3)
        if anchor.weekday()!=weekday: continue
        acc[(anchor.hour*60+anchor.minute)//G].append(c)
    prof=[];ndays=[]
    f = st.median if agg=='median' else st.mean
    for i in range(NS):
        v=acc.get(i,[]); prof.append(f(v) if v else 0.0); ndays.append(len(v))
    return prof,ndays
def slot_label(i):
    m=(i*G+3*60)%(24*60); return f"{m//60:02d}:{m%60:02d}"
def deconvolve(C,L,lam=3.0,iters=40000,lr=0.02,verbose=True):
    n=len(C); a=[max(0.0,C[i]/L) for i in range(n)]
    for it in range(iters):
        Aa=[0.0]*n;s=0.0
        for i in range(n):
            s+=a[i]
            if i-L>=0: s-=a[i-L]
            Aa[i]=s
        r=[Aa[i]-C[i] for i in range(n)]
        g=[0.0]*n;s=0.0
        for j in range(n-1,-1,-1):
            s+=r[j]
            if j+L<n: s-=r[j+L]
            g[j]=s
        for i in range(n):
            d2=0.0
            if 0<i<n-1: d2=a[i-1]-2*a[i]+a[i+1]
            g[i]+=-lam*d2*2
        mx=max(abs(x) for x in g) or 1.0
        step=lr/(1.0+it/8000)
        for i in range(n): a[i]=max(0.0,a[i]-step*g[i]/mx*max(1.0,mx)*0.02)
    Aa=[0.0]*n;s=0.0
    for i in range(n):
        s+=a[i]
        if i-L>=0: s-=a[i-L]
        Aa[i]=s
    rms=(sum((Aa[i]-C[i])**2 for i in range(n))/n)**0.5
    return a,Aa,rms
