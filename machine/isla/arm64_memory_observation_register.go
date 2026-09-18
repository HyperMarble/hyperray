// ARM64 observation names cannot shadow exact native namespace entries.
// Generated aliases are emitted only in their configured, unpadded spelling.
package isla

import (
	"strconv"
	"strings"
)

var arm64NativeRegisterNames = strings.Fields("__isla_vector_gpr __isla_continue_on_see __monomorphize_reads __monomorphize_writes VBAR_EL0 VBAR_EL1 VBAR_EL2 VBAR_EL3 CNTCR MDSCR_EL1 InGuardedPage __highest_el_aarch32 __currentInstrLength _PendingPhysicalSE __CNTControlBase HCR_EL2 TCR_EL1 TCR_EL2 TCR_EL3 TLBHits TLBMisses CFG_RMR_AA64 CFG_RVBAR CFG_ID_AA64PFR0_EL1_MPAM CFG_ID_AA64PFR0_EL1_EL3 CFG_ID_AA64PFR0_EL1_EL2 CFG_ID_AA64PFR0_EL1_EL1 CFG_ID_AA64PFR0_EL1_EL0 __v81_implemented __v82_implemented __v83_implemented __v84_implemented __v85_implemented __unpred_tsize_aborts __trickbox_enabled __tlb_enabled __syncAbortOnTTWNonCache __syncAbortOnTTWCache __syncAbortOnSoWrite __syncAbortOnSoRead __syncAbortOnReadNormNonCache __syncAbortOnReadNormCache __syncAbortOnPrefetch __syncAbortOnDeviceRead __support_52bit_pa __mte_implemented __mpam_has_hcr __vmid16_implemented __pan_implemented __fp16_implemented __dot_product_implemented __crc32_implemented __aa32_hpd_implemented __crypto_aes_implemented __crypto_sha256_implemented __crypto_sha1_implemented __syncAbortOnWriteNormNonCache __syncAbortOnWriteNormCache __syncAbortOnDeviceWrite __mpam_implemented __crypto_sm4_implemented __crypto_sm3_implemented __crypto_sha512_implemented __crypto_sha3_implemented _GTEExtObsAccess _GTEExtObsAddress _GTEExtObsData _GTEExtObsResult _GTE_PPU_SizeEn _GTE_PPU_Address _GTE_PPU_Access CPTR_EL2 CPTR_EL3 SCR_EL3 SCTLR_EL1 SCTLR_EL2 SCTLR_EL3 CPACR_EL1 ELR_EL1 ELR_EL2 ELR_EL3 ESR_EL1 FAR_EL1 SP_EL1 SP_EL2 SP_EL3 PSTATE NZCV DAIF")

func knownARM64Register(name string) bool {
	if exactARM64RegisterName(name) {
		return true
	}
	for _, prefix := range []string{"R", "X", "W"} {
		if exactARM64Alias(name, prefix, 30) {
			return true
		}
	}
	return false
}

func exactARM64RegisterName(name string) bool {
	if name == "PC" || name == "SP" || name == "_PC" || name == "SP_EL0" || name == "CPACR_EL1" {
		return true
	}
	for _, configured := range arm64NativeRegisterNames {
		if name == configured {
			return true
		}
	}
	return false
}

func exactARM64Alias(name string, prefix string, maximum uint64) bool {
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	digits := name[len(prefix):]
	index, err := strconv.ParseUint(digits, 10, 8)
	return err == nil && strconv.FormatUint(index, 10) == digits && index <= maximum
}
