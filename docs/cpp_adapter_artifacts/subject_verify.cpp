// The full replay_reader subject, with ESBMC harnesses for each extracted function.
#include <cstdint>
#include <cstddef>
#include <vector>
extern "C" size_t nondet_size_t();
extern "C" int64_t nondet_int64_t();
extern "C" unsigned nondet_uint();

static const size_t MAX_DETECTION_PREFIX_LEN = 65536;
static const size_t SCRATCH_STEP = 8192;
enum class Format { Unknown, Bam, Cram, Vcf };

size_t read_at_least(size_t want, size_t cap) {
    size_t len = 0;
    while (len < want && len < cap) {
        size_t got = SCRATCH_STEP;
        if (len + got > cap) got = cap - len;
        len = len + got;
    }
    return len;
}

class ReplayReader {
public:
    std::vector<uint8_t> prefix;
    size_t position;
    ReplayReader(std::vector<uint8_t> p, size_t pos) : prefix(p), position(pos) {}
    int64_t seek_current(int64_t offset) {
        size_t unread = prefix.size() - position;
        return offset - (int64_t)unread;
    }
    uint8_t at(size_t idx) { return prefix[idx]; }
};

int main() {
#if HARNESS == 1
    // read_at_least, bounded by the two constants stage 1 extracted
    size_t want = nondet_size_t(), cap = nondet_size_t();
    __ESBMC_assume(want <= MAX_DETECTION_PREFIX_LEN);
    __ESBMC_assume(cap  <= MAX_DETECTION_PREFIX_LEN);
    size_t r = read_at_least(want, cap);
    __ESBMC_assert(r <= cap, "invariant from stage 2: len <= cap");
#elif HARNESS == 2
    // seek_current  -- the defect
    size_t plen = nondet_size_t(), pos = nondet_size_t();
    int64_t off = nondet_int64_t();
    __ESBMC_assume(plen <= 4);
    __ESBMC_assume(pos  <= 4);
    std::vector<uint8_t> v;
    for (size_t k = 0; k < plen; k++) v.push_back(0);
    ReplayReader rr(v, pos);
    rr.seek_current(off);
#elif HARNESS == 3
    // at() -- unchecked index
    size_t plen = nondet_size_t(), idx = nondet_size_t();
    __ESBMC_assume(plen <= 4);
    __ESBMC_assume(idx  <= 4);
    std::vector<uint8_t> v;
    for (size_t k = 0; k < plen; k++) v.push_back(0);
    ReplayReader rr(v, 0);
    rr.at(idx);
#endif
    return 0;
}
