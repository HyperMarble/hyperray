#include <string>
extern "C" unsigned nondet_uint();
int main(){ std::string s = "BAM"; unsigned i = nondet_uint(); __ESBMC_assume(i<4);
  return s[i]; }   // i==3 is the NUL / out of range
