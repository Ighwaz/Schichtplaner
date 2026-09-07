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

// SlotFor liefert den Tag aus dem Plan - oder einen leeren, wenn dort noch
// nichts steht.
func SlotFor(d *AppData, date string) DaySlot {
	if slot, ok := d.Schichten[date]; ok {
		return slot
	}
	return EmptySlot()
}

// ForEachShift ruft fn für jede Schichtliste eines Tages auf.
func ForEachShift(slot *DaySlot, fn func(shift string, names *[]string)) {
	for _, shift := range allShifts {
		if f := SlotField(slot, shift); f != nil {
			fn(shift, f)
		}
	}
}

// AddToSlot trägt name in eine Schicht des Tages ein und sagt, ob sich dadurch
// etwas geändert hat - eine unbekannte Schicht oder ein Name, der schon dort
// steht, ändern nichts.
func AddToSlot(slot *DaySlot, shift, name string) bool {
	f := SlotField(slot, shift)
	if f == nil || contains(*f, name) {
		return false
	}
	*f = append(*f, name)
	return true
}

// RemoveFromSlot nimmt name aus einer Schicht heraus und sagt, ob er dort stand.
func RemoveFromSlot(slot *DaySlot, shift, name string) bool {
	f := SlotField(slot, shift)
	if f == nil || !contains(*f, name) {
		return false
	}
	*f = remove(*f, name)
	return true
}

// blockingShifts nennt die Arbeitsschichten, in denen name an diesem Tag schon
// steht und die sich nicht mit shift vertragen. Rufbereitschaft läuft neben
// allem her.
func blockingShifts(slot *DaySlot, shift, name string) []string {
	// Sie wird deshalb weder blockiert, noch blockiert sie selbst.
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

// workShifts sind die Schichten, die einander ausschließen. "normal" ist der
// Tagdienst mit Gleitzeit - er liegt zwischen Früh und Spät und schließt beide
// aus.
var workShifts = map[string]bool{
	"frueh": true, "normal": true, "spaet": true, "rufbereitschaft": true,
}

var allShifts = []string{
	"frueh", "normal", "spaet", "rufbereitschaft",
}
