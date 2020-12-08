package main

import (
	. "github.com/mmcloughlin/avo/build"
	. "github.com/mmcloughlin/avo/operand"
	. "github.com/mmcloughlin/avo/reg"
)

func AVX() {
	// Lay out the prime constant in memory
	primeData := GLOBL("prime_avx", RODATA|NOPTR)
	DATA(0, U32(2654435761))

	TEXT("accumAVX2", NOSPLIT, "func(acc *[8]uint64, data, key *byte, len uint64)")
	// %rdi, %rsi, %rdx, %rcx

	acc := Mem{Base: Load(Param("acc"), GP64())}
	data := Mem{Base: Load(Param("data"), GP64())}
	key := Mem{Base: Load(Param("key"), GP64())}
	skey := Mem{Base: Load(Param("key"), GP64())}
	plen := Load(Param("len"), GP64())
	prime := YMM()
	a := [...]VecVirtual{YMM(), YMM()}

	advance := func(n int) {
		ADDQ(U32(n*64), data.Base)
		SUBQ(U32(n*64), plen)
	}

	accum := func(doff, koff int, key Mem) {
		for n, offset := range []int{0x00, 0x20} {
			y0, y1, y2 := YMM(), YMM(), YMM()

			VMOVDQU(data.Offset(doff+offset), y0)
			VMOVDQU(key.Offset(koff+offset), y1)
			VPXOR(y0, y1, y1)
			VPSHUFD(Imm(49), y1, y2)
			VPMULUDQ(y1, y2, y1)
			VPSHUFD(Imm(78), y0, y0)
			VPADDQ(a[n], y0, a[n])
			VPADDQ(a[n], y1, a[n])
		}
	}

	scramble := func(koff int) {
		for n, offset := range []int{0x00, 0x20} {
			y0, y1 := YMM(), YMM()

			VPSRLQ(Imm(0x2f), a[n], y0)
			VPXOR(a[n], y0, y0)
			VPXOR(key.Offset(koff+offset), y0, y0)
			VPMULUDQ(prime, y0, y1)
			VPSHUFD(Imm(0xf5), y0, y0)
			VPMULUDQ(prime, y0, y0)
			VPSLLQ(Imm(0x20), y0, y0)

			VPADDQ(y1, y0, a[n])
		}
	}

	Label("load")
	{
		VMOVDQU(acc.Offset(0x00), a[0])
		VMOVDQU(acc.Offset(0x20), a[1])
		VPBROADCASTQ(primeData, prime)
	}

	Label("accum_large")
	{
		CMPQ(plen, U32(1024))
		JLE(LabelRef("accum"))

		for i := 0; i < 16; i++ {
			accum(64*i, 8*i, key)
		}
		advance(16)
		scramble(8 * 16)

		JMP(LabelRef("accum_large"))
	}

	Label("accum")
	{
		CMPQ(plen, Imm(64))
		JLE(LabelRef("finalize"))

		accum(0, 0, skey)
		advance(1)
		ADDQ(U32(8), skey.Base)

		JMP(LabelRef("accum"))
	}

	Label("finalize")
	{
		CMPQ(plen, Imm(0))
		JE(LabelRef("return"))

		SUBQ(Imm(64), data.Base)
		ADDQ(plen, data.Base)

		accum(0, 121, key)
	}

	Label("return")
	{
		VMOVDQU(a[0], acc.Offset(0x00))
		VMOVDQU(a[1], acc.Offset(0x20))
		RET()
	}

	Generate()
}
