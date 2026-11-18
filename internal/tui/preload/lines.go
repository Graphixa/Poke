package preload

import (
	"math/rand"
	"time"
)

// LineEvery is how long one loading headline stays up.
const LineEvery = 4200 * time.Millisecond

// Lines are the loading headlines. Edit this list.
// Each run is shuffled; a line does not repeat until the list wraps.
var Lines = []string{
	"Asking Slowpoke to hurry, politely",
	"Telling Snorlax this is a loading screen, not a nap",
	"Explaining to Magikarp that today is the day",
	"Checking Voltorb's ID. It says \"Item\".",
	"Hiding the Master Ball from the new hire",
	"Promising the ghost types the lights stay off",
	"Negotiating personal space with a Ditto",
	"Warning the grass types about the lawnmower",
	"Apologising to the psychic types for thinking",
	"Waiting for a Metapod to develop a personality",
	"Filing MissingNo. under \"not our problem\"",
	"Convincing Psyduck the headache is normal",
	"Asking Jigglypuff to put the microphone down",
	"Separating the Zubat from the other Zubat",
	"Teaching Splash. Again.",
	"Telling Charizard the doorway was measured",
	"Confirming Grimer is supposed to look like that",
	"Asking Team Rocket to form an orderly queue",
	"Counting Dugtrio. Lost count.",
	"Reminding Pikachu the router is not food",
	"Looking away while the ice types thaw",
	"Telling the fire types this room has sprinklers",
	"Persuading a Wobbuffet to say anything else",
	"Sorting the water types by dampness",
	"Bribing a Snorlax with one berry",
	"Explaining type matchups to a Magikarp. It splashed.",
	"Catching the one that fled. It fled again.",
	"Pretending not to notice the Ditto copying you",
}

// Shuffle returns a copy of Lines in random order.
// An empty Lines list becomes a single "Loading" headline.
func Shuffle() []string {
	if len(Lines) == 0 {
		return []string{"Loading"}
	}
	out := append([]string(nil), Lines...)
	rand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}
