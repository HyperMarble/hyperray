static const unsigned long CAP = 65536, STEP = 8192;
unsigned long scratch_len(unsigned long d) {
    __ESBMC_requires(d <= CAP);
    __ESBMC_ensures(__ESBMC_return_value <= STEP &&
                    __ESBMC_return_value + d <= CAP);
    if (d >= CAP) return 0;
    unsigned long r = CAP - d;
    return r < STEP ? r : STEP;
}
int main(){ return 0; }
