// uncaught exception escapes main
extern "C" int nondet_int();
int risky(int x) { if (x < 0) throw x; return x; }
int main() { int x = nondet_int(); __ESBMC_assume(x >= -5 && x <= 5); return risky(x); }
