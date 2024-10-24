package main

import (
	"fmt"
	"os"

	"sohio.net/surugaya/internal/compare"
)

func main() {
	oldDb := os.Args[1]
	newDb := os.Args[2]

	changes := compare.Compare(oldDb, newDb)

	for i, change := range changes {

		fmt.Printf(
			"Id: %s\nOld name: %s\nOld condition: %s\nOld price: %d~%d\nNew name: %s\nNew condition: %s\nNew price: %d~%d\n",
			change.Id,
			change.OldName,
			change.OldCondition,
			change.OldPriceLo,
			change.OldPriceHi,
			change.NewName,
			change.NewCondition,
			change.NewPriceLo,
			change.NewPriceHi,
		)

		if i != len(changes)-1 {
			fmt.Println()
		}
	}
}
