// Same code. STRONGER spec: adds the cumulative-bound row that stage 4 flagged.
extern "C" unsigned long nondet_ulong();
static const unsigned long MAX_DETECTION_PREFIX_LEN = 65536;
static const unsigned long SCRATCH_STEP = 8192;

unsigned long scratch_len(unsigned long dst_len) {
    if (dst_len >= MAX_DETECTION_PREFIX_LEN) return 0;
    unsigned long room = MAX_DETECTION_PREFIX_LEN - dst_len;
    return room < SCRATCH_STEP ? room : SCRATCH_STEP;
}

int main() {
    unsigned long d = nondet_ulong();
    __ESBMC_assume(d <= MAX_DETECTION_PREFIX_LEN);
    unsigned long r = scratch_len(d);
    __ESBMC_assert(r <= SCRATCH_STEP, "row 1: never exceeds one step");
    __ESBMC_assert(r + d <= MAX_DETECTION_PREFIX_LEN, "row 2: cumulative bound");
    __ESBMC_assert(d >= MAX_DETECTION_PREFIX_LEN ? r == 0 : r > 0, "row 3: zero iff full");
    return 0;
}
