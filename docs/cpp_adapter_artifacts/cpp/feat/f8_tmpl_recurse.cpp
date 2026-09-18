// recursive template metaprogramming (compile-time)
template<int N> struct Fact { static const int v = N * Fact<N-1>::v; };
template<> struct Fact<0> { static const int v = 1; };
int main(){ __ESBMC_assert(Fact<5>::v == 120, "5! = 120"); return 0; }
