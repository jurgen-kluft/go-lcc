package main

import (
	"encoding/binary"
	"fmt"
	"os"

	cova "github.com/jurgen-kluft/go-cova"
)

func main() {
	externMemory := make([]byte, 8)
	binary.LittleEndian.PutUint32(externMemory[4:], 45)
	script := `
extern(0) void log_alert(int data);
extern(4) int player_health;
int health_drop;

void script_main() {
	health_drop = 5;
	if ((player_health - 40) + 1) {
		log_alert(player_health);
		reduce_health(health_drop);
	}
	return;
}

void reduce_health(int delta) {
	player_health = player_health - delta;
	return;
}
`

	tokens, err := cova.Tokenize(script)
	check(err)

	program, err := cova.Parse(tokens)
	check(err)
	check(cova.Optimize(program))

	compiler := cova.NewCompiler()
	compiled, err := compiler.Compile(program)
	check(err)

	linker := cova.NewLinker(len(externMemory), 1)
	linked, err := linker.Link(program, compiled)
	check(err)

	vm := cova.NewVM(256)
	vm.BindExternBlock(externMemory)
	vm.RegisterExternDispatcher(func(vm *cova.VM, importID int) error {
		if importID != 0 {
			return fmt.Errorf("unexpected extern import id %d", importID)
		}
		value, err := vm.PopInt32()
		if err != nil {
			return err
		}
		fmt.Printf("host log_alert(%d)\n", value)
		return nil
	})

	check(vm.Run(linked))
	fmt.Printf("hostPlayerHealth=%d\n", int(int32(binary.LittleEndian.Uint32(externMemory[4:]))))
}

func check(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
