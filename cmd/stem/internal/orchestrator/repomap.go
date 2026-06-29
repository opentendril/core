package orchestrator

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	repoMapDetailLineLimit = 2000
	repoMapOutputByteLimit = 8 * 1024
	repoMapPreviewLimit    = 6
)

type repoMapFile struct {
	relPath    string
	lang       string
	lineCount  int
	signatures []string
	detailSize int
}

type repoMapNode struct {
	name     string
	isDir    bool
	children map[string]*repoMapNode
}

func newRepoMapNode(name string) *repoMapNode {
	return &repoMapNode{name: name}
}

var (
	tsClassStartRe     = regexp.MustCompile(`^(?:export\s+(?:default\s+)?)?(?:abstract\s+)?class\s+`)
	tsInterfaceStartRe = regexp.MustCompile(`^export\s+interface\s+`)
	tsFunctionStartRe  = regexp.MustCompile(`^export\s+(?:default\s+)?(?:async\s+)?function\s+`)
	tsTypeStartRe      = regexp.MustCompile(`^export\s+type\s+`)
	pythonClassStartRe = regexp.MustCompile(`^(?:async\s+)?class\s+`)
	pythonDefStartRe   = regexp.MustCompile(`^(?:async\s+)?def\s+`)
	tsMethodRe         = regexp.MustCompile(`^\s*(?:(?:public|private|protected|static|async|override|readonly)\s+)*(?:(get|set)\s+)?([A-Za-z_$][\w$]*)\s*(?:<[^>\n]*>)?\s*\(([^)]*)\)\s*(?::\s*([^;{]+))?\s*(?:\{|;)`)
	pythonClassNameRe  = regexp.MustCompile(`^class\s+([A-Za-z_][\w]*)`)
)

var repoMapSkipDirs = map[string]struct{}{
	".git":         {},
	"node_modules": {},
	"vendor":       {},
	".venv":        {},
	"venv":         {},
	"dist":         {},
	"build":        {},
	"tests":        {},
}

var repoMapSkipFileNames = map[string]struct{}{
	"conftest.py": {},
	"repomap.md":  {},
}

// GenerateRepoMap walks dir, extracts language signatures, and returns a markdown repo map.
func GenerateRepoMap(dir string) (string, error) {
	root, err := normalizeRepoMapRoot(dir)
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(root); err != nil {
		return "", err
	}

	treeRoot := newRepoMapNode(".")
	files, err := collectRepoMapFiles(root, treeRoot)
	if err != nil {
		return "", err
	}

	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].relPath) < strings.ToLower(files[j].relPath)
	})

	treeMarkdown := renderRepoMapTree(treeRoot)
	detailed := selectRepoMapDetailFiles(treeMarkdown, files)
	return renderRepoMapMarkdown(treeMarkdown, files, detailed), nil
}

func normalizeRepoMapRoot(dir string) (string, error) {
	trimmed := strings.TrimSpace(dir)
	if trimmed == "" {
		trimmed = "."
	}

	abs, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve repo map root: %w", err)
	}

	return filepath.Clean(abs), nil
}

func collectRepoMapFiles(root string, treeRoot *repoMapNode) ([]repoMapFile, error) {
	files := make([]repoMapFile, 0, 256)

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if path == root {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(filepath.Clean(rel))
		if rel == "." {
			return nil
		}

		if shouldIgnoreRepoMapPath(rel, entry.IsDir()) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		insertRepoMapPath(treeRoot, rel, entry.IsDir())

		if entry.IsDir() {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		file := repoMapFile{
			relPath:   rel,
			lang:      detectRepoMapLanguage(rel),
			lineCount: countRepoMapLines(content),
		}

		if file.lineCount < repoMapDetailLineLimit {
			file.signatures = parseRepoMapSignatures(path, content, file.lang)
		}
		if len(file.signatures) > 0 {
			file.detailSize = len(renderRepoMapFileDetail(file))
		}

		files = append(files, file)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}

func shouldIgnoreRepoMapPath(rel string, isDir bool) bool {
	normalized := filepath.ToSlash(strings.TrimSpace(rel))
	if normalized == "" {
		return true
	}

	base := strings.ToLower(filepath.Base(normalized))
	if _, ok := repoMapSkipFileNames[base]; ok && !isDir {
		return true
	}

	if isDir {
		if _, ok := repoMapSkipDirs[base]; ok {
			return true
		}
	}

	for _, segment := range strings.Split(normalized, "/") {
		lower := strings.ToLower(segment)
		if _, ok := repoMapSkipDirs[lower]; ok {
			return true
		}
	}

	if !isDir {
		if isRepoMapTestFile(base) {
			return true
		}
	}

	return false
}

func isRepoMapTestFile(base string) bool {
	lower := strings.ToLower(strings.TrimSpace(base))
	if lower == "" {
		return false
	}

	if lower == "conftest.py" {
		return true
	}

	if strings.HasSuffix(lower, "_test.go") || strings.HasSuffix(lower, "_test.ts") || strings.HasSuffix(lower, "_test.tsx") || strings.HasSuffix(lower, "_test.js") || strings.HasSuffix(lower, "_test.jsx") {
		return true
	}

	if strings.HasPrefix(lower, "test_") {
		return true
	}

	if strings.Contains(lower, ".test.") || strings.Contains(lower, ".spec.") {
		return true
	}

	return false
}

func insertRepoMapPath(root *repoMapNode, rel string, isDir bool) {
	if root == nil {
		return
	}

	parts := strings.Split(filepath.ToSlash(strings.TrimSpace(rel)), "/")
	current := root
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if current.children == nil {
			current.children = make(map[string]*repoMapNode)
		}

		child, ok := current.children[part]
		if !ok {
			child = &repoMapNode{name: part}
			current.children[part] = child
		}

		if i == len(parts)-1 {
			child.isDir = isDir
		} else {
			child.isDir = true
		}
		current = child
	}
}

func renderRepoMapTree(root *repoMapNode) string {
	if root == nil {
		return "."
	}

	lines := []string{"."}
	renderRepoMapTreeChildren(root, "", &lines)
	return strings.Join(lines, "\n")
}

func renderRepoMapTreeChildren(node *repoMapNode, prefix string, lines *[]string) {
	if node == nil || len(node.children) == 0 {
		return
	}

	children := sortedRepoMapChildren(node)
	for i, child := range children {
		last := i == len(children)-1
		branch := "|-- "
		nextPrefix := prefix + "|   "
		if last {
			branch = "`-- "
			nextPrefix = prefix + "    "
		}

		*lines = append(*lines, prefix+branch+child.name)
		if child.isDir {
			renderRepoMapTreeChildren(child, nextPrefix, lines)
		}
	}
}

func sortedRepoMapChildren(node *repoMapNode) []*repoMapNode {
	children := make([]*repoMapNode, 0, len(node.children))
	for _, child := range node.children {
		children = append(children, child)
	}

	sort.Slice(children, func(i, j int) bool {
		if children[i].isDir != children[j].isDir {
			return children[i].isDir && !children[j].isDir
		}
		ai := strings.ToLower(children[i].name)
		aj := strings.ToLower(children[j].name)
		if ai == aj {
			return children[i].name < children[j].name
		}
		return ai < aj
	})

	return children
}

func selectRepoMapDetailFiles(treeMarkdown string, files []repoMapFile) map[string]bool {
	selected := make(map[string]bool, len(files))
	candidates := make([]repoMapFile, 0, len(files))

	for _, file := range files {
		if len(file.signatures) == 0 || file.lineCount >= repoMapDetailLineLimit {
			continue
		}

		selected[file.relPath] = true
		candidates = append(candidates, file)
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].detailSize != candidates[j].detailSize {
			return candidates[i].detailSize > candidates[j].detailSize
		}
		if candidates[i].lineCount != candidates[j].lineCount {
			return candidates[i].lineCount > candidates[j].lineCount
		}
		return strings.ToLower(candidates[i].relPath) < strings.ToLower(candidates[j].relPath)
	})

	output := renderRepoMapMarkdown(treeMarkdown, files, selected)
	for len(output) > repoMapOutputByteLimit {
		demoted := false
		for _, candidate := range candidates {
			if !selected[candidate.relPath] {
				continue
			}
			delete(selected, candidate.relPath)
			demoted = true
			break
		}
		if !demoted {
			break
		}
		output = renderRepoMapMarkdown(treeMarkdown, files, selected)
	}

	return selected
}

func renderRepoMapMarkdown(treeMarkdown string, files []repoMapFile, detailed map[string]bool) string {
	var builder strings.Builder

	builder.WriteString("# Repo Map\n\n")
	builder.WriteString("## Tree\n")
	builder.WriteString(treeMarkdown)
	builder.WriteString("\n\n## Signatures\n")

	detailedFiles := make([]repoMapFile, 0, len(files))
	briefFiles := make([]repoMapFile, 0, len(files))
	for _, file := range files {
		if detailed[file.relPath] {
			detailedFiles = append(detailedFiles, file)
		} else {
			briefFiles = append(briefFiles, file)
		}
	}

	if len(detailedFiles) > 0 {
		builder.WriteString("### Detailed\n")
		for _, file := range detailedFiles {
			builder.WriteString(renderRepoMapFileDetail(file))
			builder.WriteString("\n")
		}
	}

	if len(briefFiles) > 0 {
		builder.WriteString("### Filenames Only\n")
		for _, file := range briefFiles {
			builder.WriteString("- `")
			builder.WriteString(file.relPath)
			builder.WriteString("`\n")
		}
	}

	if len(detailedFiles) == 0 && len(briefFiles) == 0 {
		builder.WriteString("- none\n")
	}

	return strings.TrimSpace(builder.String()) + "\n"
}

func renderRepoMapFileDetail(file repoMapFile) string {
	var builder strings.Builder
	builder.WriteString("### `")
	builder.WriteString(file.relPath)
	builder.WriteString("`")
	if file.lang != "" || file.lineCount > 0 {
		builder.WriteString(" (")
		if file.lang != "" {
			builder.WriteString(file.lang)
		}
		if file.lang != "" && file.lineCount > 0 {
			builder.WriteString(", ")
		}
		if file.lineCount > 0 {
			fmt.Fprintf(&builder, "%d lines", file.lineCount)
		}
		builder.WriteString(")\n")
	} else {
		builder.WriteString("\n")
	}

	if len(file.signatures) == 0 {
		builder.WriteString("- no signatures extracted\n")
		return builder.String()
	}

	for _, signature := range file.signatures {
		builder.WriteString("- ")
		builder.WriteString(signature)
		builder.WriteString("\n")
	}

	return builder.String()
}

func countRepoMapLines(content []byte) int {
	if len(content) == 0 {
		return 0
	}

	lines := bytes.Count(content, []byte{'\n'})
	if content[len(content)-1] != '\n' {
		lines++
	}
	return lines
}

func detectRepoMapLanguage(relPath string) string {
	lower := strings.ToLower(relPath)
	switch {
	case strings.HasSuffix(lower, ".go"):
		return "go"
	case strings.HasSuffix(lower, ".tsx"), strings.HasSuffix(lower, ".ts"), strings.HasSuffix(lower, ".mts"), strings.HasSuffix(lower, ".cts"), strings.HasSuffix(lower, ".d.ts"):
		return "ts"
	case strings.HasSuffix(lower, ".jsx"), strings.HasSuffix(lower, ".js"), strings.HasSuffix(lower, ".mjs"), strings.HasSuffix(lower, ".cjs"):
		return "js"
	case strings.HasSuffix(lower, ".py"):
		return "python"
	default:
		return ""
	}
}

func parseRepoMapSignatures(path string, content []byte, lang string) []string {
	switch lang {
	case "go":
		return parseGoRepoMapSignatures(path, content)
	case "ts", "js":
		return parseTSLikeRepoMapSignatures(string(content))
	case "python":
		return parsePythonRepoMapSignatures(string(content))
	default:
		return nil
	}
}

func parseGoRepoMapSignatures(path string, content []byte) []string {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, content, parser.ParseComments)
	if file == nil {
		return nil
	}

	signatures := []string{fmt.Sprintf("package %s", file.Name.Name)}
	for _, decl := range file.Decls {
		switch node := decl.(type) {
		case *ast.GenDecl:
			if node.Tok != token.TYPE {
				continue
			}
			for _, spec := range node.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				switch typeNode := typeSpec.Type.(type) {
				case *ast.StructType:
					signatures = append(signatures, renderGoStructSignature(typeSpec.Name.Name, typeNode))
				case *ast.InterfaceType:
					signatures = append(signatures, renderGoInterfaceSignature(typeSpec.Name.Name, typeNode))
				}
			}
		case *ast.FuncDecl:
			signatures = append(signatures, renderGoFuncSignature(node))
		}
	}

	if err != nil && len(signatures) == 0 {
		return nil
	}

	return signatures
}

func renderGoStructSignature(name string, node *ast.StructType) string {
	members := make([]string, 0, repoMapPreviewLimit+1)
	if node.Fields != nil {
		for _, field := range node.Fields.List {
			rendered := renderGoStructField(field)
			if rendered == "" {
				continue
			}
			members = append(members, rendered)
			if len(members) >= repoMapPreviewLimit {
				break
			}
		}
	}
	if node.Fields != nil && len(node.Fields.List) > len(members) {
		members = append(members, "...")
	}
	if len(members) == 0 {
		return fmt.Sprintf("type %s struct {}", name)
	}
	return fmt.Sprintf("type %s struct { %s }", name, strings.Join(members, "; "))
}

func renderGoInterfaceSignature(name string, node *ast.InterfaceType) string {
	members := make([]string, 0, repoMapPreviewLimit+1)
	if node.Methods != nil {
		for _, field := range node.Methods.List {
			rendered := renderGoInterfaceField(field)
			if rendered == "" {
				continue
			}
			members = append(members, rendered)
			if len(members) >= repoMapPreviewLimit {
				break
			}
		}
	}
	if node.Methods != nil && len(node.Methods.List) > len(members) {
		members = append(members, "...")
	}
	if len(members) == 0 {
		return fmt.Sprintf("type %s interface {}", name)
	}
	return fmt.Sprintf("type %s interface { %s }", name, strings.Join(members, "; "))
}

func renderGoFuncSignature(node *ast.FuncDecl) string {
	var builder strings.Builder
	builder.WriteString("func ")
	if node.Recv != nil && len(node.Recv.List) > 0 {
		builder.WriteString("(")
		builder.WriteString(renderGoFieldList(node.Recv.List))
		builder.WriteString(") ")
	}
	builder.WriteString(node.Name.Name)
	if node.Type != nil {
		builder.WriteString(renderGoTypeParameters(node.Type.TypeParams))
		builder.WriteString(renderGoGoParameters(node.Type.Params))
		builder.WriteString(renderGoGoResults(node.Type.Results))
	}
	return strings.TrimSpace(builder.String())
}

func renderGoStructField(field *ast.Field) string {
	if field == nil {
		return ""
	}

	typ := renderGoExpr(field.Type)
	if typ == "" {
		return ""
	}

	if len(field.Names) == 0 {
		return typ
	}

	names := make([]string, 0, len(field.Names))
	for _, ident := range field.Names {
		if ident != nil && strings.TrimSpace(ident.Name) != "" {
			names = append(names, ident.Name)
		}
	}
	if len(names) == 0 {
		return typ
	}

	return strings.Join(names, ", ") + " " + typ
}

func renderGoInterfaceField(field *ast.Field) string {
	if field == nil {
		return ""
	}

	if len(field.Names) == 0 {
		return renderGoExpr(field.Type)
	}

	nameParts := make([]string, 0, len(field.Names))
	for _, ident := range field.Names {
		if ident != nil && strings.TrimSpace(ident.Name) != "" {
			nameParts = append(nameParts, ident.Name)
		}
	}
	if len(nameParts) == 0 {
		return renderGoExpr(field.Type)
	}

	if fnType, ok := field.Type.(*ast.FuncType); ok {
		return strings.Join(nameParts, ", ") + renderGoGoParameters(fnType.Params) + renderGoGoResults(fnType.Results)
	}

	return strings.Join(nameParts, ", ") + " " + renderGoExpr(field.Type)
}

func renderGoFieldList(fields []*ast.Field) string {
	items := make([]string, 0, len(fields))
	for _, field := range fields {
		if field == nil {
			continue
		}
		item := renderGoStructField(field)
		if item == "" {
			continue
		}
		items = append(items, item)
	}
	return strings.Join(items, ", ")
}

func renderGoTypeParameters(fields *ast.FieldList) string {
	if fields == nil || len(fields.List) == 0 {
		return ""
	}

	items := make([]string, 0, len(fields.List))
	for _, field := range fields.List {
		if field == nil {
			continue
		}
		item := renderGoStructField(field)
		if item == "" {
			continue
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return ""
	}
	return "[" + strings.Join(items, ", ") + "]"
}

func renderGoGoParameters(fields *ast.FieldList) string {
	if fields == nil || len(fields.List) == 0 {
		return "()"
	}

	items := make([]string, 0, len(fields.List))
	for _, field := range fields.List {
		if field == nil {
			continue
		}
		item := renderGoStructField(field)
		if item == "" {
			continue
		}
		items = append(items, item)
	}
	return "(" + strings.Join(items, ", ") + ")"
}

func renderGoGoResults(fields *ast.FieldList) string {
	if fields == nil || len(fields.List) == 0 {
		return ""
	}

	items := make([]string, 0, len(fields.List))
	needsParens := len(fields.List) > 1
	for _, field := range fields.List {
		if field == nil {
			continue
		}
		item := renderGoStructField(field)
		if item == "" {
			continue
		}
		if len(field.Names) > 0 {
			needsParens = true
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return ""
	}
	if len(items) == 1 && !needsParens {
		return " " + items[0]
	}
	return " (" + strings.Join(items, ", ") + ")"
}

func renderGoExpr(expr ast.Expr) string {
	if expr == nil {
		return ""
	}

	var buf bytes.Buffer
	if err := printer.Fprint(&buf, token.NewFileSet(), expr); err != nil {
		return ""
	}
	return strings.TrimSpace(buf.String())
}

func parseTSLikeRepoMapSignatures(content string) []string {
	normalized := normalizeTSLikeSource(content)
	lines := strings.Split(normalized, "\n")
	offsets := make([]int, len(lines))
	offset := 0
	for i, line := range lines {
		offsets[i] = offset
		offset += len(line) + 1
	}

	signatures := make([]string, 0, len(lines))
	for i, line := range lines {
		if countLeadingWhitespace(line) != 0 {
			continue
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
			continue
		}

		start := offsets[i]
		switch {
		case tsInterfaceStartRe.MatchString(trimmed):
			header, body, _, ok := captureTSLikeDeclaration(normalized, start)
			if !ok {
				header = normalizeRepoMapSignature(trimmed)
			}
			signatures = append(signatures, formatTSInterfaceSignature(header, body))
		case tsClassStartRe.MatchString(trimmed):
			header, body, _, ok := captureTSLikeDeclaration(normalized, start)
			if !ok {
				header = normalizeRepoMapSignature(trimmed)
			}
			signatures = append(signatures, formatTSClassSignature(header, body))
		case tsFunctionStartRe.MatchString(trimmed):
			header, _, _, ok := captureTSLikeDeclaration(normalized, start)
			if !ok {
				header = normalizeRepoMapSignature(trimmed)
			}
			signatures = append(signatures, header)
		case tsTypeStartRe.MatchString(trimmed):
			header, body, _, ok := captureTSLikeDeclaration(normalized, start)
			if !ok {
				header = normalizeRepoMapSignature(trimmed)
			}
			signatures = append(signatures, formatTSTypeSignature(header, body))
		}
	}

	return dedupeRepoMapSignatures(signatures)
}

func normalizeTSLikeSource(content string) string {
	if strings.Contains(content, "\r") {
		content = strings.ReplaceAll(content, "\r\n", "\n")
		content = strings.ReplaceAll(content, "\r", "\n")
	}
	return content
}

func captureTSLikeDeclaration(src string, start int) (string, string, int, bool) {
	if start < 0 || start >= len(src) {
		return "", "", start, false
	}

	parenDepth := 0
	inSingle := false
	inDouble := false
	inBacktick := false
	inLineComment := false
	inBlockComment := false
	escaped := false

	for i := start; i < len(src); i++ {
		ch := src[i]
		next := byte(0)
		if i+1 < len(src) {
			next = src[i+1]
		}

		if inLineComment {
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && next == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if inSingle {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '\'' {
				inSingle = false
			}
			continue
		}
		if inDouble {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inDouble = false
			}
			continue
		}
		if inBacktick {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '`' {
				inBacktick = false
			}
			continue
		}

		switch ch {
		case '/':
			if next == '/' {
				inLineComment = true
				i++
				continue
			}
			if next == '*' {
				inBlockComment = true
				i++
				continue
			}
		case '\'':
			inSingle = true
			continue
		case '"':
			inDouble = true
			continue
		case '`':
			inBacktick = true
			continue
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '{':
			if parenDepth == 0 {
				header := normalizeRepoMapSignature(src[start:i])
				closeIndex := findMatchingTSLikeBrace(src, i)
				if closeIndex < 0 {
					return header, "", len(src), false
				}
				return header, src[i+1 : closeIndex], closeIndex + 1, true
			}
		case ';':
			if parenDepth == 0 {
				return normalizeRepoMapSignature(src[start:i]), "", i + 1, true
			}
		}
	}

	return normalizeRepoMapSignature(src[start:]), "", len(src), true
}

func findMatchingTSLikeBrace(src string, openIndex int) int {
	if openIndex < 0 || openIndex >= len(src) || src[openIndex] != '{' {
		return -1
	}

	depth := 1
	inSingle := false
	inDouble := false
	inBacktick := false
	inLineComment := false
	inBlockComment := false
	escaped := false

	for i := openIndex + 1; i < len(src); i++ {
		ch := src[i]
		next := byte(0)
		if i+1 < len(src) {
			next = src[i+1]
		}

		if inLineComment {
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && next == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if inSingle {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '\'' {
				inSingle = false
			}
			continue
		}
		if inDouble {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inDouble = false
			}
			continue
		}
		if inBacktick {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '`' {
				inBacktick = false
			}
			continue
		}

		switch ch {
		case '/':
			if next == '/' {
				inLineComment = true
				i++
				continue
			}
			if next == '*' {
				inBlockComment = true
				i++
				continue
			}
		case '\'':
			inSingle = true
			continue
		case '"':
			inDouble = true
			continue
		case '`':
			inBacktick = true
			continue
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

func formatTSInterfaceSignature(header, body string) string {
	header = normalizeRepoMapSignature(header)
	members := compactRepoMapPreviewLines(body, repoMapPreviewLimit)
	if len(members) == 0 {
		return header + " {}"
	}
	return header + " { " + strings.Join(members, "; ") + " }"
}

func formatTSClassSignature(header, body string) string {
	header = normalizeRepoMapSignature(header)
	className := extractTSClassName(header)
	methods := collectTSClassMethods(className, body)
	if len(methods) == 0 {
		return header + " {}"
	}
	return header + " { " + strings.Join(methods, "; ") + " }"
}

func formatTSTypeSignature(header, body string) string {
	header = normalizeRepoMapSignature(header)
	if body == "" {
		return header
	}

	preview := compactRepoMapPreviewLines(body, repoMapPreviewLimit)
	if len(preview) == 0 {
		return header
	}

	return header + " { " + strings.Join(preview, "; ") + " }"
}

func extractTSClassName(header string) string {
	match := regexp.MustCompile(`(?:^|\s)class\s+([A-Za-z_$][\w$]*)`).FindStringSubmatch(header)
	if len(match) == 2 && strings.TrimSpace(match[1]) != "" {
		return match[1]
	}
	return "anonymous-class"
}

func collectTSClassMethods(className, body string) []string {
	if strings.TrimSpace(body) == "" {
		return nil
	}

	lines := strings.Split(normalizeTSLikeSource(body), "\n")
	methods := make([]string, 0, repoMapPreviewLimit+1)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
			continue
		}

		match := tsMethodRe.FindStringSubmatch(trimmed)
		if len(match) != 5 {
			continue
		}

		methodName := strings.TrimSpace(match[2])
		if isRepoMapReservedKeyword(methodName) {
			continue
		}

		prefix := ""
		if strings.TrimSpace(match[1]) != "" {
			prefix = strings.TrimSpace(match[1]) + " "
		}
		params := normalizeRepoMapSignature(match[3])
		result := normalizeRepoMapSignature(match[4])

		signature := className + "." + prefix + methodName + "(" + params + ")"
		if result != "" {
			signature += ": " + result
		}
		methods = append(methods, signature)
		if len(methods) >= repoMapPreviewLimit {
			break
		}
	}

	if len(methods) == 0 {
		return nil
	}

	return methods
}

func compactRepoMapPreviewLines(body string, limit int) []string {
	lines := strings.Split(normalizeTSLikeSource(body), "\n")
	preview := make([]string, 0, limit+1)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "{" || trimmed == "}" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
			continue
		}

		normalized := normalizeRepoMapSignature(strings.TrimSuffix(strings.TrimSuffix(trimmed, ";"), ","))
		if normalized == "" {
			continue
		}
		preview = append(preview, normalized)
		if len(preview) >= limit {
			break
		}
	}

	if len(preview) == 0 {
		return nil
	}

	if len(preview) == limit && len(lines) > limit {
		preview = append(preview, "...")
	}

	return preview
}

func parsePythonRepoMapSignatures(content string) []string {
	normalized := normalizeTSLikeSource(content)
	lines := strings.Split(normalized, "\n")
	signatures := make([]string, 0, len(lines))
	classStack := make([]pythonClassContext, 0, 8)

	for i := 0; i < len(lines); {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			i++
			continue
		}

		indent := countLeadingWhitespace(line)
		for len(classStack) > 0 && indent <= classStack[len(classStack)-1].indent {
			classStack = classStack[:len(classStack)-1]
		}

		switch {
		case pythonClassStartRe.MatchString(trimmed):
			sig, nextIndex, ok := capturePythonSignature(lines, i)
			if !ok {
				sig = normalizeRepoMapSignature(trimmed)
				nextIndex = i + 1
			}

			className := extractPythonClassName(sig)
			signatures = append(signatures, sig)
			classStack = append(classStack, pythonClassContext{name: className, indent: indent})
			i = nextIndex
			continue
		case pythonDefStartRe.MatchString(trimmed):
			sig, nextIndex, ok := capturePythonSignature(lines, i)
			if !ok {
				sig = normalizeRepoMapSignature(trimmed)
				nextIndex = i + 1
			}

			if len(classStack) > 0 && indent > classStack[len(classStack)-1].indent {
				signatures = append(signatures, prefixPythonMethodSignature(classStack[len(classStack)-1].name, sig))
			} else {
				signatures = append(signatures, sig)
			}
			i = nextIndex
			continue
		default:
			i++
		}
	}

	return dedupeRepoMapSignatures(signatures)
}

type pythonClassContext struct {
	name   string
	indent int
}

func capturePythonSignature(lines []string, start int) (string, int, bool) {
	if start < 0 || start >= len(lines) {
		return "", start, false
	}

	parenDepth := 0
	bracketDepth := 0
	braceDepth := 0
	inSingle := false
	inDouble := false
	inTripleSingle := false
	inTripleDouble := false
	escaped := false

	var builder strings.Builder
	for i := start; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if i > start {
			builder.WriteByte(' ')
		}
		builder.WriteString(trimmed)

		for j := 0; j < len(line); j++ {
			ch := line[j]
			next := byte(0)
			if j+1 < len(line) {
				next = line[j+1]
			}
			next2 := byte(0)
			if j+2 < len(line) {
				next2 = line[j+2]
			}

			if !inTripleSingle && !inTripleDouble {
				if ch == '#' && !inSingle && !inDouble {
					break
				}
			}

			if inTripleSingle {
				if ch == '\'' && next == '\'' && next2 == '\'' {
					inTripleSingle = false
					j += 2
				}
				continue
			}
			if inTripleDouble {
				if ch == '"' && next == '"' && next2 == '"' {
					inTripleDouble = false
					j += 2
				}
				continue
			}
			if inSingle {
				if escaped {
					escaped = false
					continue
				}
				if ch == '\\' {
					escaped = true
					continue
				}
				if ch == '\'' {
					inSingle = false
				}
				continue
			}
			if inDouble {
				if escaped {
					escaped = false
					continue
				}
				if ch == '\\' {
					escaped = true
					continue
				}
				if ch == '"' {
					inDouble = false
				}
				continue
			}

			switch ch {
			case '\'':
				if next == '\'' && next2 == '\'' {
					inTripleSingle = true
					j += 2
				} else {
					inSingle = true
				}
			case '"':
				if next == '"' && next2 == '"' {
					inTripleDouble = true
					j += 2
				} else {
					inDouble = true
				}
			case '(':
				parenDepth++
			case ')':
				if parenDepth > 0 {
					parenDepth--
				}
			case '[':
				bracketDepth++
			case ']':
				if bracketDepth > 0 {
					bracketDepth--
				}
			case '{':
				braceDepth++
			case '}':
				if braceDepth > 0 {
					braceDepth--
				}
			case ':':
				if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 {
					signature := normalizePythonRepoMapSignature(builder.String())
					if signature == "" {
						signature = normalizePythonRepoMapSignature(trimmed)
					}
					return trimTrailingColon(signature), i + 1, true
				}
			}
		}
	}

	signature := normalizePythonRepoMapSignature(builder.String())
	if signature == "" {
		return "", start, false
	}
	return trimTrailingColon(signature), len(lines), true
}

func extractPythonClassName(signature string) string {
	match := pythonClassNameRe.FindStringSubmatch(strings.TrimSpace(signature))
	if len(match) == 2 && strings.TrimSpace(match[1]) != "" {
		return match[1]
	}
	return "anonymous-class"
}

func prefixPythonMethodSignature(className, signature string) string {
	sig := normalizePythonRepoMapSignature(signature)
	if sig == "" {
		return className
	}

	asyncPrefix := ""
	if strings.HasPrefix(sig, "async def ") {
		asyncPrefix = "async "
		sig = strings.TrimPrefix(sig, "async def ")
	} else if strings.HasPrefix(sig, "def ") {
		sig = strings.TrimPrefix(sig, "def ")
	}

	sig = trimTrailingColon(sig)
	return normalizePythonRepoMapSignature(asyncPrefix + className + "." + sig)
}

func trimTrailingColon(signature string) string {
	return strings.TrimSpace(strings.TrimSuffix(signature, ":"))
}

func normalizePythonRepoMapSignature(signature string) string {
	signature = normalizeRepoMapSignature(signature)
	signature = strings.ReplaceAll(signature, "( ", "(")
	signature = strings.ReplaceAll(signature, "[ ", "[")
	signature = strings.ReplaceAll(signature, "{ ", "{")
	signature = strings.ReplaceAll(signature, " ,", ",")
	signature = strings.ReplaceAll(signature, ", )", ")")
	signature = strings.ReplaceAll(signature, " )", ")")
	signature = strings.ReplaceAll(signature, " ) ->", ") ->")
	signature = strings.ReplaceAll(signature, " ) :", ") :")
	return normalizeRepoMapSignature(signature)
}

func normalizeRepoMapSignature(signature string) string {
	signature = strings.TrimSpace(signature)
	if signature == "" {
		return ""
	}
	signature = strings.ReplaceAll(signature, "\r", " ")
	signature = strings.Join(strings.Fields(signature), " ")
	signature = strings.TrimSpace(signature)
	signature = strings.TrimSuffix(signature, "{")
	signature = strings.TrimSuffix(signature, ";")
	signature = strings.TrimSpace(signature)
	return signature
}

func countLeadingWhitespace(line string) int {
	count := 0
	for _, r := range line {
		switch r {
		case ' ':
			count++
		case '\t':
			count += 4
		default:
			return count
		}
	}
	return count
}

func isRepoMapReservedKeyword(word string) bool {
	switch strings.TrimSpace(strings.ToLower(word)) {
	case "if", "for", "while", "switch", "catch", "return", "const", "let", "var", "function", "class", "new", "try", "else", "do", "case", "default", "constructor":
		return true
	default:
		return false
	}
}

func dedupeRepoMapSignatures(signatures []string) []string {
	if len(signatures) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(signatures))
	deduped := make([]string, 0, len(signatures))
	for _, sig := range signatures {
		normalized := normalizeRepoMapSignature(sig)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		deduped = append(deduped, normalized)
	}

	return deduped
}
