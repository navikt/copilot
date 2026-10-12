#!/usr/bin/env python3
"""Utvalgsstørrelse for full pinnemåling (trinn 4 i docs/pin-protokoll.md).

Ensidig ikke-underlegenhet for to andeler: formel-n, simulert styrke og alfa
med Newcombe hybrid score-intervall, og sekvensiell test med tre kikk
(O'Brien–Fleming-type) ved 24/48/72 kjøringer per modell. Tallene står i
alternativnotatet i https://github.com/navikt/copilot/issues/1414.
Fast frø (1), så utskriften er lik fra gang til gang.
«sim n» er første n der simulert styrke (20 000 trekk) er minst 0,8. Det er et
anslag med Monte Carlo-støy på omtrent ±0,005, ikke en eksakt minste n: for
90 %, 5 pp gir eksakt binomisk summering styrke 0,7987 ved n = 447.

Kjør: uv run --with numpy python scripts/pin-utvalgsstorrelse.py
"""
import numpy as np, math
from statistics import NormalDist as N
rng=np.random.default_rng(1)
za=N().inv_cdf(.95); zb=N().inv_cdf(.8)
def nf(p,d): return math.ceil((za+zb)**2*2*p*(1-p)/d**2)
assert nf(.95,.10)==59
for p in (.95,.90):
  print(p,[nf(p,d) for d in (.05,.10,.15)])
def wilson(x,n,z=za):
  ph=x/n; c=z*z/n; den=1+c; mid=(ph+c/2)/den; h=z*np.sqrt(ph*(1-ph)/n+c/(4*n))/den
  return mid-h, mid+h
def power(p,d,n,sims=40000):
  # Newcombe hybrid score, one-sided 95%; new = arm b, need lower(pb-pa) > -d
  a=rng.binomial(n,p,sims); b=rng.binomial(n,p,sims)
  la,ua=wilson(a,n); lb,ub=wilson(b,n)
  pa=a/n; pb=b/n
  low=(pb-pa)-np.sqrt((pb-lb)**2+(ua-pa)**2)
  return float((low>-d).mean())
def alpha(p,d,n,sims=40000):
  a=rng.binomial(n,p,sims); b=rng.binomial(n,p-d,sims)
  la,ua=wilson(a,n); lb,ub=wilson(b,n); pa=a/n; pb=b/n
  low=(pb-pa)-np.sqrt((pb-lb)**2+(ua-pa)**2); return float((low>-d).mean())
for p,d in ((.95,.10),(.90,.15),(.90,.05)):
  n=nf(p,d); print('cell',p,d,n,'power',power(p,d,n),'alpha',alpha(p,d,n))
  for m in range(5,800):
    if power(p,d,m,20000)>=.8: print(' sim n for .8:',m); break
# Sekvensiell test: tre kikk, Newcombe-grense med z_k = 1,645 * sqrt(3/k) ved kikk k.
# Stopper som bestått når nedre grense > -d, som «ikke avgjort» når differansen < -d ved kikk 1 eller 2.
def gs(p_ref,p_new,d,nmax,sims=20000):
  looks=[round(nmax*k/3) for k in (1,2,3)]; zk=[za*math.sqrt(3/k) for k in (1,2,3)]
  used=[];win=0
  for _ in range(sims):
    a=rng.binomial(1,p_ref,nmax).cumsum(); b=rng.binomial(1,p_new,nmax).cumsum()
    for i,(n,z) in enumerate(zip(looks,zk)):
      x,y=a[n-1],b[n-1]
      la,ua=wilson(x,n,z); lb,ub=wilson(y,n,z); pa=x/n; pb=y/n
      low=(pb-pa)-math.sqrt((pb-lb)**2+(ua-pa)**2)
      if low>-d: win+=1; used.append(n); break
      if i<2 and (pb-pa)< -d: used.append(n); break  # futility: clearly worse
      if i==2: used.append(n)
  return float(np.mean(used)), win/sims
for nmax in (72,):
  for pn in (.95,.90,.85,.65):
    print('gs .95 d.10 nmax',nmax,'pnew',pn, gs(.95,pn,.10,nmax))
# Resten av cellene i tabellen. Kjøres sist, så tallene over er uendret.
for p,d in ((.95,.05),(.95,.15),(.90,.10)):
  n=nf(p,d); print('cell',p,d,n,'power',power(p,d,n),'alpha',alpha(p,d,n))
  for m in range(5,800):
    if power(p,d,m,20000)>=.8: print(' sim n for .8:',m); break
