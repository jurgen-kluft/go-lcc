package main

import (
	"fmt"
	"os"

	lcc "github.com/jurgen-kluft/go-lcc"
)

func main() {
	hostPlayerHealth := 45
	script := `
global(0) void log_alert(int data);
global(0) int player_health;

void script_main() {
	if ((player_health - 40) + 1) {
		log_alert(player_health);
		player_health = player_health - 5;
	}
	return;
}
`

	tokens, err := lcc.Tokenize(script)
	check(err)

	program, err := lcc.Parse(tokens)
	check(err)

	compiler := lcc.NewCompiler()
	compiled, err := compiler.Compile(program)
	check(err)

	linker := lcc.NewLinker(1, 1)
	linked, err := linker.Link(program, compiled)
	check(err)

	vm := lcc.NewVM(1, linked.LocalSlotCount)
	check(vm.BindGlobal(0, &hostPlayerHealth))
	vm.RegisterFunction(0, func(vm *lcc.VM) error {
		fmt.Printf("host log_alert(%d)\n", vm.Arg(0))
		return nil
	})

	check(vm.Run(linked))
	fmt.Printf("hostPlayerHealth=%d\n", hostPlayerHealth)
}

func check(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
