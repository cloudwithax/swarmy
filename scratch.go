package main
import "fmt"
import "path/filepath"
import "strings"
func main() {
    candidate := "/home/user/outside.go"
    workingDir := "/repo"
    
    cleaned := filepath.Clean(candidate)
    fmt.Println("IsAbs:", filepath.IsAbs(cleaned))
    fmt.Println("HasPrefix /:", strings.HasPrefix(cleaned, "/"))
    fmt.Println("HasPrefix \\:", strings.HasPrefix(cleaned, "\\"))
    
    rel, err := filepath.Rel(filepath.Clean(workingDir), cleaned)
    fmt.Println("Rel:", rel, "err:", err)
}
