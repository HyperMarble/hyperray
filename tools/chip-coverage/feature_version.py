#!/usr/bin/env python3
# Which ARM architecture version each feature was introduced in.
# Source: Arm Architecture Reference Manual for A-profile, section A2
# "Architectural features", which lists every FEAT_ name with its version.
# A feature not listed here is reported as unknown, never guessed.
INTRODUCED = {
    "FEAT_CRC32": "8.0", "FEAT_AES": "8.0", "FEAT_SHA1": "8.0", "FEAT_SHA256": "8.0",
    "FEAT_PMULL": "8.0", "AdvSIMD": "8.0", "AdvSIMD_HPFPCvt": "8.0",
    "FEAT_CSV2": "8.0", "FEAT_CSV3": "8.0", "FP_SyncExceptions": "8.0",
    "FEAT_LSE": "8.1", "FEAT_RDM": "8.1",
    "FEAT_FP16": "8.2", "FEAT_DotProd": "8.2", "FEAT_FHM": "8.2", "FEAT_SHA3": "8.2",
    "FEAT_SHA512": "8.2", "FEAT_DPB": "8.2", "FEAT_LSE2": "8.2", "FEAT_I8MM": "8.2",
    "FEAT_BF16": "8.2",
    "FEAT_PAuth": "8.3", "FEAT_PACIMP": "8.3", "FEAT_LRCPC": "8.3", "FEAT_FCMA": "8.3",
    "FEAT_JSCVT": "8.3",
    "FEAT_FlagM": "8.4", "FEAT_LRCPC2": "8.4", "FEAT_DIT": "8.4",
    "FEAT_FlagM2": "8.5", "FEAT_SB": "8.5", "FEAT_FRINTTS": "8.5", "FEAT_DPB2": "8.5",
    "FEAT_BTI": "8.5", "FEAT_PAuth2": "8.5", "FEAT_FPAC": "8.5", "FEAT_FPACCOMBINE": "8.5",
    "FEAT_ECV": "8.6",
    "FEAT_WFxT": "8.7", "FEAT_RPRES": "8.7", "FEAT_AFP": "8.7",
    "FEAT_SME": "9.2", "FEAT_SME2": "9.2",
    "FEAT_SME_F64F64": "9.2", "FEAT_SME_I16I64": "9.2",
    "SME_F32F32": "9.2", "SME_BI32I32": "9.2", "SME_B16F32": "9.2",
    "SME_F16F32": "9.2", "SME_I8I32": "9.2", "SME_I16I32": "9.2",
}


def introduced_in(feature: str) -> str:
    """The ARM version a feature first appeared in, or empty if unknown."""
    return INTRODUCED.get(feature, "")
