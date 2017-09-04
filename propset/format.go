package propset

import (
	"fmt"
	"io"
	"sort"
	"text/tabwriter"
)

func Fprint(out io.Writer, pset PropSet) {

	names := make([]string, 0, len(pset))
	for k, _ := range pset {
		names = append(names, k)
	}
	sort.Strings(names)

	table := tabwriter.NewWriter(out, 0, 8, 2, ' ', 0)

	for _, name := range names {
		prop := pset[name]

		fmt.Fprintf(table, "%v\t", name)

		switch prop := prop.(type) {

		case Map:

			if len(prop) == 0 {
				fmt.Fprintf(table, "{}", name)
				continue
			}

			first := true
			for k, v := range prop {
				if first {
					fmt.Fprintf(table, "%v\t%v\n", k, v)
					first = false
					continue
				}
				fmt.Fprintf(table, "\t%v\t%v\n", k, v)
			}
		default:
			fmt.Fprintf(table, "%v\n", prop)
		}
	}

	table.Flush()
}
