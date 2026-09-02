// templates: does ESBMC verify through a monomorphized template?
#include <cstdint>
extern "C" int32_t nondet_int32();
template <typename T> T clamp_add(T a, T b, T hi) { T s = a + b; return s > hi ? hi : s; }
int main() {
    int32_t a = nondet_int32(), b = nondet_int32();
    __ESBMC_assume(a >= 0 && a <= 1000);
    __ESBMC_assume(b >= 0 && b <= 1000);
    int32_t r = clamp_add<int32_t>(a, b, 100);
    __ESBMC_assert(r <= 100, "clamped");
    return 0;
}
