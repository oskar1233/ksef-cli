package render

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"

	ksef "github.com/oskar1233/ksef/internal"
)

type XMLField struct {
	Path  string
	Value string
}

type xmlNode struct {
	Name     string
	Attrs    []XMLField
	Text     string
	Children []*xmlNode
}

type invoiceDocument struct {
	InvoiceNumber string
	KSeFNumber    string
	InvoiceType   string
	Currency      string
	SubjectLabel  string
	Summary       []kvRow
	Details       []kvRow
	Seller        partySection
	Buyer         partySection
	Positions     []positionRow
	TaxSummary    []taxSummaryRow
	Payment       []kvRow
	Registers     []kvRow
	Footer        []string
	Others        []kvRow
	TotalGross    string
}

type kvRow struct {
	Label string
	Value string
}

type partySection struct {
	Lines []string
}

type positionRow struct {
	LP       string
	Name     string
	UnitNet  string
	Qty      string
	Unit     string
	TaxRate  string
	NetValue string
}

type taxSummaryRow struct {
	TaxRate string
	Net     string
	Tax     string
	Gross   string
	TaxPLN  string
}

func buildInvoiceDocument(root *xmlNode, invoice ksef.InvoiceMetadata, subjectLabel string) invoiceDocument {
	fields, _ := ExtractXMLFieldsFromRoot(root)
	used := map[string]struct{}{}

	fa := root.child("Fa")
	stopka := root.child("Stopka")
	naglowek := root.child("Naglowek")

	invoiceNumber := firstNonEmpty(markUsed(used, "Faktura/Fa/P_2", valueAtPath(root, "Faktura/Fa/P_2")), invoice.InvoiceNumber)
	currency := firstNonEmpty(markUsed(used, "Faktura/Fa/KodWaluty", valueAtPath(root, "Faktura/Fa/KodWaluty")), invoice.Currency)
	invoiceType := invoiceTypeLabel(firstNonEmpty(markUsed(used, "Faktura/Fa/RodzajFaktury", valueAtPath(root, "Faktura/Fa/RodzajFaktury")), invoice.InvoiceType))

	doc := invoiceDocument{
		InvoiceNumber: invoiceNumber,
		KSeFNumber:    invoice.KSeFNumber,
		InvoiceType:   invoiceType,
		Currency:      currency,
		SubjectLabel:  subjectLabelText(subjectLabel),
		Summary: []kvRow{
			{Label: "Numer faktury", Value: invoiceNumber},
			{Label: "Numer KSeF", Value: invoice.KSeFNumber},
			{Label: "Rodzaj faktury", Value: invoiceType},
			{Label: "Data wystawienia", Value: firstNonEmpty(markUsed(used, "Faktura/Fa/P_1", valueAtPath(root, "Faktura/Fa/P_1")), invoice.IssueDate)},
			{Label: "Waluta", Value: currency},
		},
		Details:    buildDetailsSection(root, invoice, naglowek, fa, used),
		Seller:     buildPartySection(root.child("Podmiot1"), "Sprzedawca", used, "Faktura/Podmiot1"),
		Buyer:      buildPartySection(root.child("Podmiot2"), "Nabywca", used, "Faktura/Podmiot2"),
		Positions:  buildPositions(fa, currency, used),
		TaxSummary: buildTaxSummary(root, currency, used),
		Payment:    buildPaymentSection(fa, used),
		Registers:  buildRegisterSection(stopka, used),
		Footer:     buildFooterLines(stopka, used),
		TotalGross: formatMoney(firstNonEmpty(markUsed(used, "Faktura/Fa/P_15", valueAtPath(root, "Faktura/Fa/P_15")), fmt.Sprintf("%.2f", invoice.GrossAmount)), currency),
	}

	doc.Others = buildOtherFields(fields, used)
	return doc
}

func buildDetailsSection(root *xmlNode, invoice ksef.InvoiceMetadata, naglowek *xmlNode, fa *xmlNode, used map[string]struct{}) []kvRow {
	rows := []kvRow{}

	issueDate := firstNonEmpty(markUsed(used, "Faktura/Fa/P_1", valueAtPath(root, "Faktura/Fa/P_1")), invoice.IssueDate)
	if issueDate != "" {
		rows = append(rows, kvRow{Label: "Data wystawienia", Value: issueDate})
	}

	periodFrom := markUsed(used, "Faktura/Fa/OkresFa/P_6_Od", valueAtPath(root, "Faktura/Fa/OkresFa/P_6_Od"))
	periodTo := markUsed(used, "Faktura/Fa/OkresFa/P_6_Do", valueAtPath(root, "Faktura/Fa/OkresFa/P_6_Do"))
	if periodFrom != "" || periodTo != "" {
		rows = append(rows, kvRow{Label: "Data dokonania lub zakończenia dostawy towarów lub wykonania usługi", Value: strings.TrimSpace(fmt.Sprintf("od %s do %s", periodFrom, periodTo))})
	} else {
		deliveryDate := markUsed(used, "Faktura/Fa/P_6", valueAtPath(root, "Faktura/Fa/P_6"))
		if deliveryDate != "" {
			rows = append(rows, kvRow{Label: "Data dokonania lub zakończenia dostawy towarów lub wykonania usługi", Value: deliveryDate})
		}
	}

	if currency := markUsed(used, "Faktura/Fa/KodWaluty", valueAtPath(root, "Faktura/Fa/KodWaluty")); currency != "" {
		rows = append(rows, kvRow{Label: "Kod waluty", Value: currency})
	}

	return compactRows(rows)
}

func buildPartySection(node *xmlNode, sectionName string, used map[string]struct{}, basePath string) partySection {
	_ = sectionName
	if node == nil {
		return partySection{}
	}

	lines := make([]string, 0)
	if nip := markUsed(used, basePath+"/DaneIdentyfikacyjne/NIP", valueAtPath(node, node.Name+"/DaneIdentyfikacyjne/NIP")); nip != "" {
		lines = append(lines, "NIP: "+nip)
	}
	if name := markUsed(used, basePath+"/DaneIdentyfikacyjne/Nazwa", valueAtPath(node, node.Name+"/DaneIdentyfikacyjne/Nazwa")); name != "" {
		lines = append(lines, "Nazwa: "+name)
	}

	addressLines := partyAddressLines(node, basePath+"/Adres", used)
	if len(addressLines) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Adres")
		lines = append(lines, addressLines...)
	}

	contactLines := partyContactLines(node, basePath+"/DaneKontaktowe", used)
	if len(contactLines) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Dane kontaktowe")
		lines = append(lines, contactLines...)
	}

	if nrKlienta := markUsed(used, basePath+"/NrKlienta", valueAtPath(node, node.Name+"/NrKlienta")); nrKlienta != "" {
		lines = append(lines, "")
		lines = append(lines, "Numer klienta: "+nrKlienta)
	}
	if jst := markUsed(used, basePath+"/JST", valueAtPath(node, node.Name+"/JST")); jst != "" {
		lines = append(lines, fmt.Sprintf("Faktura dotyczy jednostki podrzędnej JST: %s", yesNoFromCode(jst)))
	}
	if gv := markUsed(used, basePath+"/GV", valueAtPath(node, node.Name+"/GV")); gv != "" {
		lines = append(lines, fmt.Sprintf("Faktura dotyczy członka grupy GV: %s", yesNoFromCode(gv)))
	}

	return partySection{Lines: lines}
}

func partyAddressLines(node *xmlNode, basePath string, used map[string]struct{}) []string {
	address := node.child("Adres")
	if address == nil {
		return nil
	}
	lines := make([]string, 0)
	for _, label := range []string{"AdresL1", "AdresL2", "AdresL3", "AdresL4"} {
		path := basePath + "/" + label
		if value := markUsed(used, path, valueAtPath(node, node.Name+"/Adres/"+label)); value != "" {
			lines = append(lines, value)
		}
	}
	if country := markUsed(used, basePath+"/KodKraju", valueAtPath(node, node.Name+"/Adres/KodKraju")); country != "" {
		lines = append(lines, countryLabel(country))
	}
	return lines
}

func partyContactLines(node *xmlNode, basePath string, used map[string]struct{}) []string {
	contact := node.child("DaneKontaktowe")
	if contact == nil {
		return nil
	}
	labelMap := map[string]string{"Email": "E-mail", "Telefon": "Telefon"}
	lines := make([]string, 0)
	for _, child := range contact.Children {
		path := basePath + "/" + child.Name
		value := markUsed(used, path, strings.TrimSpace(child.Text))
		if value == "" {
			continue
		}
		label := labelMap[child.Name]
		if label == "" {
			label = child.Name
		}
		lines = append(lines, fmt.Sprintf("%s: %s", label, value))
	}
	return lines
}

func buildPositions(fa *xmlNode, currency string, used map[string]struct{}) []positionRow {
	_ = currency
	if fa == nil {
		return nil
	}
	children := fa.children("FaWiersz")
	rows := make([]positionRow, 0, len(children))
	for index, child := range children {
		base := fmt.Sprintf("Faktura/Fa/FaWiersz[%d]", index+1)
		rows = append(rows, positionRow{
			LP:       firstNonEmpty(markUsed(used, base+"/NrWierszaFa", valueAtPath(child, child.Name+"/NrWierszaFa")), fmt.Sprintf("%d", index+1)),
			Name:     firstNonEmpty(markUsed(used, base+"/P_7", valueAtPath(child, child.Name+"/P_7")), markUsed(used, base+"/NazwaTowaruLubUslugi", valueAtPath(child, child.Name+"/NazwaTowaruLubUslugi"))),
			UnitNet:  chooseFirstUsed(used, []string{base + "/P_9A", base + "/P_9B", base + "/P_11", base + "/P_11A"}, child, []string{"P_9A", "P_9B", "P_11", "P_11A"}),
			Qty:      chooseFirstUsed(used, []string{base + "/P_8B", base + "/Ilosc"}, child, []string{"P_8B", "Ilosc"}),
			Unit:     chooseFirstUsed(used, []string{base + "/P_8A", base + "/Miara"}, child, []string{"P_8A", "Miara"}),
			TaxRate:  addPercentIfNeeded(chooseFirstUsed(used, []string{base + "/P_12", base + "/StawkaPodatku"}, child, []string{"P_12", "StawkaPodatku"})),
			NetValue: chooseFirstUsed(used, []string{base + "/P_11", base + "/P_11A", base + "/WartoscSprzedazyNetto"}, child, []string{"P_11", "P_11A", "WartoscSprzedazyNetto"}),
		})
	}
	return rows
}

func buildTaxSummary(root *xmlNode, currency string, used map[string]struct{}) []taxSummaryRow {
	_ = currency
	rateLabels := []struct {
		Suffix string
		Label  string
	}{
		{Suffix: "1", Label: "23% lub 22%"},
		{Suffix: "2", Label: "8% lub 7%"},
		{Suffix: "3", Label: "5%"},
		{Suffix: "4", Label: "0%"},
		{Suffix: "5", Label: "zw"},
		{Suffix: "6", Label: "oo"},
		{Suffix: "7", Label: "np"},
	}

	rows := make([]taxSummaryRow, 0)
	for _, rate := range rateLabels {
		net := markUsed(used, "Faktura/Fa/P_13_"+rate.Suffix, valueAtPath(root, "Faktura/Fa/P_13_"+rate.Suffix))
		tax := markUsed(used, "Faktura/Fa/P_14_"+rate.Suffix, valueAtPath(root, "Faktura/Fa/P_14_"+rate.Suffix))
		taxPLN := markUsed(used, "Faktura/Fa/P_14_"+rate.Suffix+"W", valueAtPath(root, "Faktura/Fa/P_14_"+rate.Suffix+"W"))
		if net == "" && tax == "" && taxPLN == "" {
			continue
		}
		gross := ""
		if net != "" && tax != "" {
			gross = sumDecimalStrings(net, tax)
		}
		rows = append(rows, taxSummaryRow{TaxRate: rate.Label, Net: net, Tax: tax, Gross: gross, TaxPLN: taxPLN})
	}
	return rows
}

func buildPaymentSection(fa *xmlNode, used map[string]struct{}) []kvRow {
	rows := make([]kvRow, 0)
	if fa == nil {
		return rows
	}
	payment := fa.child("Platnosc")
	if payment == nil {
		return rows
	}

	if paid := markUsed(used, "Faktura/Fa/Platnosc/Zaplacono", valueAtPath(fa, "Fa/Platnosc/Zaplacono")); paid != "" {
		rows = append(rows, kvRow{Label: "Zapłacono", Value: yesNoFromCode(paid)})
	} else {
		rows = append(rows, kvRow{Label: "Informacja o płatności", Value: "Brak zapłaty"})
	}
	if paymentOther := markUsed(used, "Faktura/Fa/Platnosc/PlatnoscInna", valueAtPath(fa, "Fa/Platnosc/PlatnoscInna")); paymentOther != "" {
		rows = append(rows, kvRow{Label: "Płatność inna", Value: yesNoFromCode(paymentOther)})
	}
	if paymentDate := markUsed(used, "Faktura/Fa/Platnosc/DataZaplaty", valueAtPath(fa, "Fa/Platnosc/DataZaplaty")); paymentDate != "" {
		rows = append(rows, kvRow{Label: "Data zapłaty", Value: paymentDate})
	}
	if paymentDeadline := markUsed(used, "Faktura/Fa/Platnosc/TerminPlatnosci/Termin", valueAtPath(fa, "Fa/Platnosc/TerminPlatnosci/Termin")); paymentDeadline != "" {
		rows = append(rows, kvRow{Label: "Termin płatności", Value: paymentDeadline})
	}
	if paymentForm := markUsed(used, "Faktura/Fa/Platnosc/FormaPlatnosci", valueAtPath(fa, "Fa/Platnosc/FormaPlatnosci")); paymentForm != "" {
		rows = append(rows, kvRow{Label: "Forma płatności", Value: paymentFormLabel(paymentForm)})
	}
	if description := markUsed(used, "Faktura/Fa/Platnosc/OpisPlatnosci", valueAtPath(fa, "Fa/Platnosc/OpisPlatnosci")); description != "" {
		rows = append(rows, kvRow{Label: "Opis płatności", Value: description})
	}
	if account := markUsed(used, "Faktura/Fa/Platnosc/RachunekBankowy/NrRB", valueAtPath(fa, "Fa/Platnosc/RachunekBankowy/NrRB")); account != "" {
		rows = append(rows, kvRow{Label: "Numer rachunku", Value: account})
	}
	if swift := markUsed(used, "Faktura/Fa/Platnosc/RachunekBankowy/SWIFT", valueAtPath(fa, "Fa/Platnosc/RachunekBankowy/SWIFT")); swift != "" {
		rows = append(rows, kvRow{Label: "SWIFT", Value: swift})
	}
	if bankName := markUsed(used, "Faktura/Fa/Platnosc/RachunekBankowy/NazwaBanku", valueAtPath(fa, "Fa/Platnosc/RachunekBankowy/NazwaBanku")); bankName != "" {
		rows = append(rows, kvRow{Label: "Bank", Value: bankName})
	}
	return compactRows(rows)
}

func buildRegisterSection(stopka *xmlNode, used map[string]struct{}) []kvRow {
	if stopka == nil {
		return nil
	}
	registers := stopka.child("Rejestry")
	if registers == nil {
		return nil
	}
	rows := make([]kvRow, 0)
	for _, child := range registers.Children {
		value := markUsed(used, "Faktura/Stopka/Rejestry/"+child.Name, strings.TrimSpace(child.Text))
		if value == "" {
			continue
		}
		rows = append(rows, kvRow{Label: registerLabel(child.Name), Value: value})
	}
	return rows
}

func buildFooterLines(stopka *xmlNode, used map[string]struct{}) []string {
	if stopka == nil {
		return nil
	}
	lines := make([]string, 0)
	for index, info := range stopka.children("Informacje") {
		path := fmt.Sprintf("Faktura/Stopka/Informacje[%d]/StopkaFaktury", index+1)
		value := markUsed(used, path, valueAtPath(info, "Informacje/StopkaFaktury"))
		if value == "" {
			continue
		}
		lines = append(lines, value)
	}
	return lines
}

func buildOtherFields(fields []XMLField, used map[string]struct{}) []kvRow {
	rows := make([]kvRow, 0)
	for _, field := range fields {
		if field.Value == "" {
			continue
		}
		if _, exists := used[field.Path]; exists {
			continue
		}
		if isSimpleCode(field.Value) {
			continue
		}
		if strings.Contains(field.Path, "@") || strings.Contains(field.Path, "KodFormularza") || strings.Contains(field.Path, "WariantFormularza") {
			continue
		}
		rows = append(rows, kvRow{Label: prettifyPath(field.Path), Value: field.Value})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].Label < rows[j].Label
	})
	if len(rows) > 20 {
		rows = rows[:20]
	}
	return rows
}

func ExtractXMLFields(xmlContent []byte) ([]XMLField, error) {
	root, err := parseXML(xmlContent)
	if err != nil {
		return nil, err
	}
	return ExtractXMLFieldsFromRoot(root)
}

func ExtractXMLFieldsFromRoot(root *xmlNode) ([]XMLField, error) {
	fields := make([]XMLField, 0)
	flattenXML(root, root.Name, &fields)
	fields = deduplicateFields(fields)
	return fields, nil
}

func parseXML(content []byte) (*xmlNode, error) {
	decoder := xml.NewDecoder(bytes.NewReader(content))
	stack := make([]*xmlNode, 0)
	var root *xmlNode

	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("decode xml: %w", err)
		}

		switch value := token.(type) {
		case xml.StartElement:
			node := &xmlNode{Name: xmlName(value.Name)}
			for _, attr := range value.Attr {
				node.Attrs = append(node.Attrs, XMLField{Path: "@" + xmlName(attr.Name), Value: strings.TrimSpace(attr.Value)})
			}
			if len(stack) == 0 {
				root = node
			} else {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, node)
			}
			stack = append(stack, node)
		case xml.CharData:
			if len(stack) == 0 {
				continue
			}
			text := strings.TrimSpace(string(value))
			if text == "" {
				continue
			}
			node := stack[len(stack)-1]
			if node.Text == "" {
				node.Text = text
			} else {
				node.Text += " " + text
			}
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}

	if root == nil {
		return nil, fmt.Errorf("empty xml document")
	}
	return root, nil
}

func flattenXML(node *xmlNode, path string, fields *[]XMLField) {
	for _, attr := range node.Attrs {
		*fields = append(*fields, XMLField{Path: path + attr.Path, Value: attr.Value})
	}
	if strings.TrimSpace(node.Text) != "" {
		*fields = append(*fields, XMLField{Path: path, Value: strings.TrimSpace(node.Text)})
	}
	if len(node.Children) == 0 {
		return
	}

	occurrences := map[string]int{}
	totals := map[string]int{}
	for _, child := range node.Children {
		totals[child.Name]++
	}
	for _, child := range node.Children {
		occurrences[child.Name]++
		childPath := path + "/" + child.Name
		if totals[child.Name] > 1 {
			childPath = fmt.Sprintf("%s[%d]", childPath, occurrences[child.Name])
		}
		flattenXML(child, childPath, fields)
	}
}

func deduplicateFields(fields []XMLField) []XMLField {
	seen := map[string]struct{}{}
	result := make([]XMLField, 0, len(fields))
	for _, field := range fields {
		key := field.Path + "\x00" + field.Value
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, field)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Path < result[j].Path
	})
	return result
}

func (n *xmlNode) child(name string) *xmlNode {
	if n == nil {
		return nil
	}
	for _, child := range n.Children {
		if child.Name == name {
			return child
		}
	}
	return nil
}

func (n *xmlNode) children(name string) []*xmlNode {
	if n == nil {
		return nil
	}
	result := make([]*xmlNode, 0)
	for _, child := range n.Children {
		if child.Name == name {
			result = append(result, child)
		}
	}
	return result
}

func valueAtPath(root *xmlNode, path string) string {
	if root == nil {
		return ""
	}
	parts := strings.Split(path, "/")
	current := root
	if len(parts) > 0 && parts[0] == current.Name {
		parts = parts[1:]
	}
	for _, part := range parts {
		name := stripIndex(part)
		current = current.child(name)
		if current == nil {
			return ""
		}
	}
	return strings.TrimSpace(current.Text)
}

func chooseFirstUsed(used map[string]struct{}, paths []string, node *xmlNode, names []string) string {
	for index, name := range names {
		value := valueAtPath(node, node.Name+"/"+name)
		if strings.TrimSpace(value) == "" {
			continue
		}
		return markUsed(used, paths[index], value)
	}
	return ""
}

func markUsed(used map[string]struct{}, path string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	used[path] = struct{}{}
	return value
}

func compactRows(rows []kvRow) []kvRow {
	result := make([]kvRow, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.Value) == "" {
			continue
		}
		result = append(result, row)
	}
	return result
}

func countryLabel(code string) string {
	countries := map[string]string{
		"PL": "Polska",
		"DE": "Niemcy",
		"LU": "Luksemburg",
		"CZ": "Czechy",
		"SK": "Słowacja",
	}
	if value, exists := countries[strings.ToUpper(code)]; exists {
		return value
	}
	return code
}

func yesNoFromCode(value string) string {
	switch strings.TrimSpace(value) {
	case "1":
		return "TAK"
	case "2":
		return "NIE"
	default:
		return value
	}
}

func invoiceTypeLabel(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "VAT":
		return "Faktura podstawowa"
	case "KOR":
		return "Faktura korygująca"
	case "ZAL":
		return "Faktura zaliczkowa"
	default:
		if value == "" {
			return "Faktura"
		}
		return value
	}
}

func paymentFormLabel(value string) string {
	switch strings.TrimSpace(value) {
	case "1":
		return "Gotówka"
	case "2":
		return "Karta"
	case "3":
		return "Bon"
	case "4":
		return "Czek"
	case "5":
		return "Kredyt"
	case "6":
		return "Przelew"
	case "7":
		return "Mobilna"
	default:
		return value
	}
}

func registerLabel(name string) string {
	labels := map[string]string{
		"PelnaNazwa": "Pełna nazwa",
		"BDO":        "BDO",
	}
	if label, exists := labels[name]; exists {
		return label
	}
	return name
}

func prettifyPath(path string) string {
	path = strings.TrimPrefix(path, "Faktura/")
	path = strings.ReplaceAll(path, "[", " ")
	path = strings.ReplaceAll(path, "]", "")
	path = strings.ReplaceAll(path, "/", " → ")
	return path
}

func addPercentIfNeeded(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "%") {
		return value
	}
	return value + "%"
}

func formatMoney(value string, currency string) string {
	value = strings.TrimSpace(value)
	currency = strings.TrimSpace(currency)
	if value == "" {
		return ""
	}
	if currency == "" {
		return value
	}
	return value + " " + currency
}

func sumDecimalStrings(a string, b string) string {
	var af, bf float64
	fmt.Sscanf(strings.ReplaceAll(a, ",", "."), "%f", &af)
	fmt.Sscanf(strings.ReplaceAll(b, ",", "."), "%f", &bf)
	return fmt.Sprintf("%.2f", af+bf)
}

func isSimpleCode(value string) bool {
	value = strings.TrimSpace(value)
	return value == "1" || value == "2"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stripIndex(part string) string {
	if index := strings.Index(part, "["); index >= 0 {
		return part[:index]
	}
	return part
}

func xmlName(name xml.Name) string {
	if name.Local != "" {
		return name.Local
	}
	return name.Space
}

func subjectLabelText(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "purchase":
		return "Zakup"
	case "sales":
		return "Sprzedaż"
	default:
		return strings.TrimSpace(value)
	}
}
