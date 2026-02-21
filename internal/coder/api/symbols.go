package api

// GetCode extracts the source code for a named symbol from the original source bytes.
// Supports both bare names ("add_numbers") and qualified names ("MyStruct.Method").
// It never uses cached data — always reads from the live source.
func GetCode(source []byte, symbols []Symbol, name string) (string, bool) {
	for _, sym := range symbols {
		if sym.Name == name {
			if sym.StartByte >= 0 && sym.EndByte <= len(source) && sym.StartByte < sym.EndByte {
				return string(source[sym.StartByte:sym.EndByte]), true
			}
			return "", false
		}
		for _, m := range sym.Methods {
			if m.Name == name || sym.Name+"."+m.Name == name {
				if m.StartByte >= 0 && m.EndByte <= len(source) && m.StartByte < m.EndByte {
					return string(source[m.StartByte:m.EndByte]), true
				}
				return "", false
			}
		}
	}
	return "", false
}

// GetDoc returns the documentation string for a named symbol.
// Supports both bare names ("add_numbers") and qualified names ("MyStruct.Method").
func GetDoc(symbols []Symbol, name string) (string, bool) {
	for _, sym := range symbols {
		if sym.Name == name {
			return sym.Doc, sym.Doc != ""
		}
		for _, m := range sym.Methods {
			if m.Name == name || sym.Name+"."+m.Name == name {
				return m.Doc, m.Doc != ""
			}
		}
	}
	return "", false
}
