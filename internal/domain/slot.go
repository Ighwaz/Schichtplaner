package domain

func SlotField(s *DaySlot, shift string) *[]string {
	switch shift {
	case "frueh":
		return &s.Frueh
	case "normal":
		return &s.Normal
	case "spaet":
		return &s.Spaet
	case "rufbereitschaft":
		return &s.Rufbereitschaft
	}
	return nil
}

// SlotFor returns the day's slot, or an empty one if the day has no entries yet.
func SlotFor(d *AppData, date string) DaySlot {
	if slot, ok := d.Schichten[date]; ok {
		return slot
	}
	return EmptySlot()
}

// ForEachShift calls fn for every shift list of a slot.
func ForEachShift(slot *DaySlot, fn func(shift string, names *[]string)) {
	for _, shift := range allShifts {
		if f := SlotField(slot, shift); f != nil {
			fn(shift, f)
		}
	}
}

// AddToSlot adds name to one shift of a slot and reports whether that changed
// anything - an unknown shift or a name that is already there changes nothing.
func AddToSlot(slot *DaySlot, shift, name string) bool {
	f := SlotField(slot, shift)
	if f == nil || contains(*f, name) {
		return false
	}
	*f = append(*f, name)
	return true
}

// RemoveFromSlot removes name from one shift and reports whether it was there.
func RemoveFromSlot(slot *DaySlot, shift, name string) bool {
	f := SlotField(slot, shift)
	if f == nil || !contains(*f, name) {
		return false
	}
	*f = remove(*f, name)
	return true
}

// blockingShifts lists the work shifts name already holds that day and that
// cannot be combined with shift. Rufbereitschaft runs alongside everything.
func blockingShifts(slot *DaySlot, shift, name string) []string {
	// Rufbereitschaft laeuft neben jeder Arbeitsschicht her und wird deshalb
	// weder blockiert noch blockiert sie selbst.
	if !workShifts[shift] || shift == "rufbereitschaft" {
		return nil
	}
	var blocking []string
	for _, s := range allShifts {
		if !workShifts[s] || s == shift || s == "rufbereitschaft" {
			continue
		}
		if f := SlotField(slot, s); f != nil && contains(*f, name) {
			blocking = append(blocking, s)
		}
	}
	return blocking
}

func contains(arr []string, s string) bool {
	for _, v := range arr {
		if v == s {
			return true
		}
	}
	return false
}

func remove(arr []string, s string) []string {
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		if v != s {
			out = append(out, v)
		}
	}
	return out
}

// workShifts sind die Schichten, die miteinander kollidieren koennen.
// "normal" ist der Tagdienst mit Gleitzeit - er liegt zwischen Frueh und
// Spaet und schliesst beide aus.
var workShifts = map[string]bool{
	"frueh": true, "normal": true, "spaet": true, "rufbereitschaft": true,
}

var allShifts = []string{
	"frueh", "normal", "spaet", "rufbereitschaft",
}
