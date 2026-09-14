# BUGS

- When resizing the tui, the sixel cache will start creating the new sixel sizes for the cache for the bigger size (expected), but when you resize the terminal down the sixel cache work halts (it shoudl continue in the background)
- config edit get's stuck after run ```go run .\cmd\poke\main.go config edit
PS D:\Data\Programming\Pokedex> exit status 0xc000013a```
