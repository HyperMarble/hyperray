extern "C" unsigned long nondet_ulong();
static const unsigned long CAP = 65536, STEP = 8192;
unsigned long orig(unsigned long d){ if(d>=CAP) return 0; unsigned long r=CAP-d; return r<STEP?r:STEP; }
unsigned long mut1(unsigned long d){ if(d> CAP) return 0; unsigned long r=CAP-d; return r<STEP?r:STEP; }
unsigned long mut2(unsigned long d){ if(d>=CAP) return 0; unsigned long r=CAP-d; return r<=STEP?r:STEP; }
int main(){ unsigned long d=nondet_ulong(); __ESBMC_assume(d<=CAP);
  __ESBMC_assert(orig(d)==mut1(d), "mut1 equivalent");
  __ESBMC_assert(orig(d)==mut2(d), "mut2 equivalent"); return 0; }
