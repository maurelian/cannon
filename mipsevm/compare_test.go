package main

import (
	"fmt"
	uc "github.com/unicorn-engine/unicorn/bindings/go/unicorn"
	"log"
	"sync"
	"testing"
	"time"
)

func RegSerialize(ram map[uint32](uint32)) []uint32 {
	ret := []uint32{ram[0xc0000080], uint32(len(ram))}
	// 36 registers, 32 basic + pc + hi/lo + heap
	for i := uint32(0xc0000000); i < 0xc0000000+36*4; i += 4 {
		ret = append(ret, ram[i])
	}
	return ret
}

var done sync.Mutex

func TestCompare(t *testing.T) {
	fn := "../mipigo/test/test.bin"
	//fn := "../mipigo/minigeth.bin"

	steps := 1000000000
	//steps := 1165
	//steps := 1180
	//steps := 24

	cevm := make(chan []uint32, 1)
	cuni := make(chan []uint32, 1)

	evmram := make(map[uint32](uint32))
	LoadMappedFile(fn, evmram, 0)
	inputFile := fmt.Sprintf("/tmp/eth/%d", 13284469)
	LoadMappedFile(inputFile, evmram, 0xB0000000)
	// init registers to 0 in evm
	for i := uint32(0xC0000000); i < 0xC0000000+36*4; i += 4 {
		WriteRam(evmram, i, 0)
	}

	go RunWithRam(evmram, steps, 0, func(step int, ram map[uint32](uint32)) {
		//fmt.Printf("%d evm %x\n", step, ram[0xc0000080])
		cevm <- RegSerialize(ram)
		done.Lock()
		done.Unlock()
	})

	uniram := make(map[uint32](uint32))
	go RunUnicorn(fn, uniram, steps, func(step int, mu uc.Unicorn, ram map[uint32](uint32)) {
		SyncRegs(mu, ram)
		cuni <- RegSerialize(ram)
		done.Lock()
		done.Unlock()
	})

	mismatch := false
	for i := 0; i < steps; i++ {
		x, y := <-cevm, <-cuni
		if x[0] == 0x5ead0000 && y[0] == 0x5ead0000 {
			fmt.Println("both processes exited")
			break
		}
		if i%100000 == 0 {
			fmt.Println(i, x[0:9], y[0:9])
		}
		for j := 0; j < len(x); j++ {
			if x[j] != y[j] {
				fmt.Println(i, "mismatch at", j, "cevm", x, "cuni", y)
				mismatch = true
			}
		}
		if mismatch {
			break
		}
	}

	// final ram check
	done.Lock()
	time.Sleep(100 * time.Millisecond)
	for k, v := range uniram {
		if val, ok := evmram[k]; !ok || val != v {
			fmt.Printf("ram mismatch at %x, evm %x != uni %x\n", k, evmram[k], uniram[k])
			mismatch = true
		}
	}
	if mismatch {
		log.Fatal("RAM mismatch")
	}
}
