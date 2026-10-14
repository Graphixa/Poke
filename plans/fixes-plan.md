# BUGS

- Make the Home key jump to the top and End key jump to the bottom of the list.
- Move the pokemon Total number to the top Right corner of the pokedex border [ 1024 / 1024 ] - Consider 1024 of 1024 and maybe only when filter is on, if not filtering don't show at all.
- When resizing the tui, the sixel cache will start creating the new sixel sizes for the cache for the bigger size (expected), but when you resize the terminal down the sixel cache work halts (it shoudl continue in the background)

- config edit get's stuck after run ```go run .\cmd\poke\main.go config edit

PS D:\Data\Programming\Pokedex> exit status 0xc000013a```

