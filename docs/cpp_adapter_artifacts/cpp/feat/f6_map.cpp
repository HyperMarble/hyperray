#include <map>
extern "C" int nondet_int();
int main(){ std::map<int,int> m; m[1]=10; int k=nondet_int(); __ESBMC_assume(k>=1&&k<=2);
  __ESBMC_assert(m[k]==10,"all keys map to 10"); return 0; }
