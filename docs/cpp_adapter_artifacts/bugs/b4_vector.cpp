// b4: std::vector out-of-range via operator[]
#include <vector>
#include <cstddef>
extern "C" unsigned nondet_uint();
int main() {
    std::vector<int> v;
    v.push_back(1); v.push_back(2); v.push_back(3);
    unsigned i = nondet_uint();
    __ESBMC_assume(i < 4);            // i==3 is out of range for size 3
    return v[i];
}
