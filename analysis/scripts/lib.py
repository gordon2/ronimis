import csv, glob, os, datetime as dt, statistics as st
from collections import defaultdict

RT = os.path.expanduser("~/Library/Application Support/ronimis")

def load(loc, pat="gym-stats-202608*.csv"):
    rows=[]
    for f in sorted(glob.glob(os.path.join(RT,pat))):
        with open(f,newline='') as fh:
            for rec in csv.DictReader(fh):
                if rec.get('status')!='success' or rec.get('location_name')!=loc: continue
                try:
                    ts=dt.datetime.strptime(rec['timestamp'],'%Y-%m-%d %H:%M:%S'); c=int(rec['user_count'])
                except Exception: continue
                rows.append((ts,c))
    rows.sort(); return rows

STEP = 2  # минут: шаг опроса коллектора

def to_grid(rows):
    """Регуляризация на сетку STEP минут: dict[datetime_slot] = count (медиана если дубли)."""
    g = defaultdict(list)
    for ts,c in rows:
        slot = ts.replace(second=0, microsecond=0)
        slot = slot.replace(minute=(slot.minute//STEP)*STEP)
        g[slot].append(c)
    return {k: int(round(st.median(v))) for k,v in sorted(g.items())}

def fill_gaps(grid):
    """Заполнить пропуски сетки линейной интерполяцией; вернуть непрерывный список (slot, count)."""
    if not grid: return []
    ks = sorted(grid)
    out = []
    cur = ks[0]; end = ks[-1]
    d = dt.timedelta(minutes=STEP)
    while cur <= end:
        out.append((cur, grid.get(cur)))
        cur += d
    # интерполяция
    for i,(t,v) in enumerate(out):
        if v is not None: continue
        j = i-1
        while j>=0 and out[j][1] is None: j-=1
        k = i+1
        while k<len(out) and out[k][1] is None: k+=1
        if j>=0 and k<len(out):
            a=out[j][1]; b=out[k][1]
            out[i]=(t, int(round(a+(b-a)*(i-j)/(k-j))))
        elif j>=0: out[i]=(t,out[j][1])
        elif k<len(out): out[i]=(t,out[k][1])
        else: out[i]=(t,0)
    return out
