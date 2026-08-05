//go:build amd64

#include "textflag.h"

// shuffleBGRA maps every 4-byte group to out[0]=in[2], out[1]=in[1],
// out[2]=in[0], out[3]=in[3]. No index has its high bit set, so the constant
// stays inside the signed range the assembler accepts for an 8-byte DATA word.
// It is duplicated across both 128-bit halves because VPSHUFB shuffles each
// lane independently.
DATA shuffleBGRA<>+0x00(SB)/8, $0x0704050603000102
DATA shuffleBGRA<>+0x08(SB)/8, $0x0F0C0D0E0B08090A
DATA shuffleBGRA<>+0x10(SB)/8, $0x0704050603000102
DATA shuffleBGRA<>+0x18(SB)/8, $0x0F0C0D0E0B08090A
GLOBL shuffleBGRA<>(SB), RODATA|NOPTR, $32

// The opaque kernels build their 0xff000000 alpha constant with
// VPCMPEQD + VPSLLD rather than loading it, which keeps every DATA word in
// range and costs two instructions once per call.

// func swizzleOpaqueAVX2(dst, src []byte)
TEXT ·swizzleOpaqueAVX2(SB), NOSPLIT, $0-48
	MOVQ    dst_base+0(FP), DI
	MOVQ    dst_len+8(FP), BX
	MOVQ    src_base+24(FP), SI
	MOVQ    src_len+32(FP), CX
	CMPQ    CX, BX
	CMOVQLT CX, BX
	ANDQ    $-32, BX
	JZ      opaqueAVX2Done

	VMOVDQU  shuffleBGRA<>(SB), Y2
	VPCMPEQD Y3, Y3, Y3
	VPSLLD   $24, Y3, Y3
	XORQ     AX, AX

	MOVQ BX, DX
	ANDQ $-128, DX
	JZ   opaqueAVX2Tail

opaqueAVX2Block:
	VMOVDQU (SI)(AX*1), Y0
	VMOVDQU 32(SI)(AX*1), Y1
	VMOVDQU 64(SI)(AX*1), Y4
	VMOVDQU 96(SI)(AX*1), Y5
	VPSHUFB Y2, Y0, Y0
	VPSHUFB Y2, Y1, Y1
	VPSHUFB Y2, Y4, Y4
	VPSHUFB Y2, Y5, Y5
	VPOR    Y3, Y0, Y0
	VPOR    Y3, Y1, Y1
	VPOR    Y3, Y4, Y4
	VPOR    Y3, Y5, Y5
	VMOVDQU Y0, (DI)(AX*1)
	VMOVDQU Y1, 32(DI)(AX*1)
	VMOVDQU Y4, 64(DI)(AX*1)
	VMOVDQU Y5, 96(DI)(AX*1)
	ADDQ    $128, AX
	CMPQ    AX, DX
	JLT     opaqueAVX2Block

opaqueAVX2Tail:
	CMPQ AX, BX
	JGE  opaqueAVX2End

opaqueAVX2TailLoop:
	VMOVDQU (SI)(AX*1), Y0
	VPSHUFB Y2, Y0, Y0
	VPOR    Y3, Y0, Y0
	VMOVDQU Y0, (DI)(AX*1)
	ADDQ    $32, AX
	CMPQ    AX, BX
	JLT     opaqueAVX2TailLoop

opaqueAVX2End:
	VZEROUPPER

opaqueAVX2Done:
	RET

// func swizzleOpaqueAVX2NT(dst, src []byte)
//
// dst must be 32-byte aligned; the Go caller splits any unaligned head off
// first. SFENCE orders the weakly ordered stores before the frame is handed to
// another goroutine.
TEXT ·swizzleOpaqueAVX2NT(SB), NOSPLIT, $0-48
	MOVQ    dst_base+0(FP), DI
	MOVQ    dst_len+8(FP), BX
	MOVQ    src_base+24(FP), SI
	MOVQ    src_len+32(FP), CX
	CMPQ    CX, BX
	CMOVQLT CX, BX
	ANDQ    $-32, BX
	JZ      opaqueNTDone

	VMOVDQU  shuffleBGRA<>(SB), Y2
	VPCMPEQD Y3, Y3, Y3
	VPSLLD   $24, Y3, Y3
	XORQ     AX, AX

	MOVQ BX, DX
	ANDQ $-128, DX
	JZ   opaqueNTTail

opaqueNTBlock:
	VMOVDQU  (SI)(AX*1), Y0
	VMOVDQU  32(SI)(AX*1), Y1
	VMOVDQU  64(SI)(AX*1), Y4
	VMOVDQU  96(SI)(AX*1), Y5
	VPSHUFB  Y2, Y0, Y0
	VPSHUFB  Y2, Y1, Y1
	VPSHUFB  Y2, Y4, Y4
	VPSHUFB  Y2, Y5, Y5
	VPOR     Y3, Y0, Y0
	VPOR     Y3, Y1, Y1
	VPOR     Y3, Y4, Y4
	VPOR     Y3, Y5, Y5
	VMOVNTDQ Y0, (DI)(AX*1)
	VMOVNTDQ Y1, 32(DI)(AX*1)
	VMOVNTDQ Y4, 64(DI)(AX*1)
	VMOVNTDQ Y5, 96(DI)(AX*1)
	ADDQ     $128, AX
	CMPQ     AX, DX
	JLT      opaqueNTBlock

opaqueNTTail:
	CMPQ AX, BX
	JGE  opaqueNTEnd

opaqueNTTailLoop:
	VMOVDQU  (SI)(AX*1), Y0
	VPSHUFB  Y2, Y0, Y0
	VPOR     Y3, Y0, Y0
	VMOVNTDQ Y0, (DI)(AX*1)
	ADDQ     $32, AX
	CMPQ     AX, BX
	JLT      opaqueNTTailLoop

opaqueNTEnd:
	SFENCE
	VZEROUPPER

opaqueNTDone:
	RET

// func swizzleKeepAlphaAVX2(dst, src []byte)
TEXT ·swizzleKeepAlphaAVX2(SB), NOSPLIT, $0-48
	MOVQ    dst_base+0(FP), DI
	MOVQ    dst_len+8(FP), BX
	MOVQ    src_base+24(FP), SI
	MOVQ    src_len+32(FP), CX
	CMPQ    CX, BX
	CMOVQLT CX, BX
	ANDQ    $-32, BX
	JZ      keepAVX2Done

	VMOVDQU shuffleBGRA<>(SB), Y2
	XORQ    AX, AX

	MOVQ BX, DX
	ANDQ $-128, DX
	JZ   keepAVX2Tail

keepAVX2Block:
	VMOVDQU (SI)(AX*1), Y0
	VMOVDQU 32(SI)(AX*1), Y1
	VMOVDQU 64(SI)(AX*1), Y4
	VMOVDQU 96(SI)(AX*1), Y5
	VPSHUFB Y2, Y0, Y0
	VPSHUFB Y2, Y1, Y1
	VPSHUFB Y2, Y4, Y4
	VPSHUFB Y2, Y5, Y5
	VMOVDQU Y0, (DI)(AX*1)
	VMOVDQU Y1, 32(DI)(AX*1)
	VMOVDQU Y4, 64(DI)(AX*1)
	VMOVDQU Y5, 96(DI)(AX*1)
	ADDQ    $128, AX
	CMPQ    AX, DX
	JLT     keepAVX2Block

keepAVX2Tail:
	CMPQ AX, BX
	JGE  keepAVX2End

keepAVX2TailLoop:
	VMOVDQU (SI)(AX*1), Y0
	VPSHUFB Y2, Y0, Y0
	VMOVDQU Y0, (DI)(AX*1)
	ADDQ    $32, AX
	CMPQ    AX, BX
	JLT     keepAVX2TailLoop

keepAVX2End:
	VZEROUPPER

keepAVX2Done:
	RET

// func swizzleOpaqueSSSE3(dst, src []byte)
TEXT ·swizzleOpaqueSSSE3(SB), NOSPLIT, $0-48
	MOVQ    dst_base+0(FP), DI
	MOVQ    dst_len+8(FP), BX
	MOVQ    src_base+24(FP), SI
	MOVQ    src_len+32(FP), CX
	CMPQ    CX, BX
	CMOVQLT CX, BX
	ANDQ    $-16, BX
	JZ      opaqueSSSE3Done

	MOVOU   shuffleBGRA<>(SB), X2
	PCMPEQL X3, X3
	PSLLL   $24, X3
	XORQ    AX, AX

	MOVQ BX, DX
	ANDQ $-64, DX
	JZ   opaqueSSSE3Tail

opaqueSSSE3Block:
	MOVOU  (SI)(AX*1), X0
	MOVOU  16(SI)(AX*1), X1
	MOVOU  32(SI)(AX*1), X4
	MOVOU  48(SI)(AX*1), X5
	PSHUFB X2, X0
	PSHUFB X2, X1
	PSHUFB X2, X4
	PSHUFB X2, X5
	POR    X3, X0
	POR    X3, X1
	POR    X3, X4
	POR    X3, X5
	MOVOU  X0, (DI)(AX*1)
	MOVOU  X1, 16(DI)(AX*1)
	MOVOU  X4, 32(DI)(AX*1)
	MOVOU  X5, 48(DI)(AX*1)
	ADDQ   $64, AX
	CMPQ   AX, DX
	JLT    opaqueSSSE3Block

opaqueSSSE3Tail:
	CMPQ AX, BX
	JGE  opaqueSSSE3Done

opaqueSSSE3TailLoop:
	MOVOU  (SI)(AX*1), X0
	PSHUFB X2, X0
	POR    X3, X0
	MOVOU  X0, (DI)(AX*1)
	ADDQ   $16, AX
	CMPQ   AX, BX
	JLT    opaqueSSSE3TailLoop

opaqueSSSE3Done:
	RET

// func swizzleKeepAlphaSSSE3(dst, src []byte)
TEXT ·swizzleKeepAlphaSSSE3(SB), NOSPLIT, $0-48
	MOVQ    dst_base+0(FP), DI
	MOVQ    dst_len+8(FP), BX
	MOVQ    src_base+24(FP), SI
	MOVQ    src_len+32(FP), CX
	CMPQ    CX, BX
	CMOVQLT CX, BX
	ANDQ    $-16, BX
	JZ      keepSSSE3Done

	MOVOU shuffleBGRA<>(SB), X2
	XORQ  AX, AX

	MOVQ BX, DX
	ANDQ $-64, DX
	JZ   keepSSSE3Tail

keepSSSE3Block:
	MOVOU  (SI)(AX*1), X0
	MOVOU  16(SI)(AX*1), X1
	MOVOU  32(SI)(AX*1), X4
	MOVOU  48(SI)(AX*1), X5
	PSHUFB X2, X0
	PSHUFB X2, X1
	PSHUFB X2, X4
	PSHUFB X2, X5
	MOVOU  X0, (DI)(AX*1)
	MOVOU  X1, 16(DI)(AX*1)
	MOVOU  X4, 32(DI)(AX*1)
	MOVOU  X5, 48(DI)(AX*1)
	ADDQ   $64, AX
	CMPQ   AX, DX
	JLT    keepSSSE3Block

keepSSSE3Tail:
	CMPQ AX, BX
	JGE  keepSSSE3Done

keepSSSE3TailLoop:
	MOVOU  (SI)(AX*1), X0
	PSHUFB X2, X0
	MOVOU  X0, (DI)(AX*1)
	ADDQ   $16, AX
	CMPQ   AX, BX
	JLT    keepSSSE3TailLoop

keepSSSE3Done:
	RET
