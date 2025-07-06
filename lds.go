package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

var (
	logger = log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: true,
		TimeFormat:      time.Kitchen,
	})

	// Colors and styles
	dirStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#4e9a06")) // green
	fileStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#729fcf")) // blue
	hiddenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#888a85")) // gray
	linkStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#ad7fa8")) // purple
	permStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#f57900")) // orange
	sizeStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#3465a4")) // darker blue
	modTimeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#cc0000")) // red
	resetStyle   = lipgloss.NewStyle()
)

const (
	branch = "╰─ "
	pipe   = "│  "
	tee    = "├─ "
	space  = "   "
)

type Options struct {
	showHidden bool
	longFormat bool
	reverse    bool
	derefLinks bool
	noSymlink  bool
	maxDepth   int
}

func main() {
	opts := &Options{}

	args := os.Args[1:]
	paths := []string{}

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "--help" || arg == "-h" {
			fmt.Println("Usage: lds [options] [path ...]")
			fmt.Println("Options:")
			fmt.Println("  -a            Show hidden files.")
			fmt.Println("  -l 			 Show long listing.")
			fmt.Println("  -r            Reverse order.")
			fmt.Println("  -L            Follow symlinks.")
			fmt.Println("  --no-symlink  Do not follow symlinks.")
			return
		}

		if arg == "--no-symlink" {
			opts.noSymlink = true
			continue
		}

		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			for _, ch := range arg[1:] {
				switch ch {
				case 'a':
					opts.showHidden = true
				case 'l':
					opts.longFormat = true
				case 'r':
					opts.reverse = true
				case 'L':
					opts.derefLinks = true
				default:
					logger.Fatalf("Unknown option: -%c", ch)
				}
			}
			continue
		}

		paths = append(paths, arg)
	}

	if len(paths) == 0 {
		paths = append(paths, ".")
	}

	for i, root := range paths {
		if len(paths) > 1 {
			if i > 0 {
				fmt.Println()
			}
			fmt.Println(root + ":")
		}
		err := printTree(root, opts, "", true)
		if err != nil {
			logger.Warnf("Error listing %s: %v", root, err)
		}
	}
}

func printTree(path string, opts *Options, prefix string, isRoot bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}

	entries, err := readDir(path, opts.showHidden, opts.reverse)
	if err != nil {
		return err
	}

	if isRoot {
		name := filepath.Base(path)
		if name == "." {
			name = path
		}
		printNode(name, info, opts, "", true, true, "", false)
	}

	for i, entry := range entries {
		isLast := i == len(entries)-1
		entryPath := filepath.Join(path, entry.Name())

		entryIsSymlink := entry.Mode()&os.ModeSymlink != 0
		linkTarget := ""

		if entryIsSymlink && opts.derefLinks {
			targetPath, err := filepath.EvalSymlinks(entryPath)
			if err == nil {
				targetInfo, err := os.Stat(targetPath)
				if err == nil {
					entry = targetInfo
					entryPath = targetPath
					entryIsSymlink = false
				}
			}
		} else if entryIsSymlink && !opts.noSymlink {
			target, err := os.Readlink(entryPath)
			if err == nil {
				linkTarget = target
			}
		}

		linePrefix := prefix
		if isLast {
			linePrefix += branch
		} else {
			linePrefix += tee
		}

		printNode(entry.Name(), entry, opts, linePrefix, false, true, linkTarget, entryIsSymlink)

		if entry.IsDir() {
			var newPrefix string
			if isLast {
				newPrefix = prefix + space
			} else {
				newPrefix = prefix + pipe
			}
			err := printTree(entryPath, opts, newPrefix, false)
			if err != nil {
				logger.Warnf("Error reading directory %s: %v", entryPath, err)
			}
		}
	}

	return nil
}

func printNode(name string, info fs.FileInfo, opts *Options, prefix string, isRoot bool, printName bool, linkTarget string, isSymlink bool) {
	isDir := info.IsDir()
	hidden := strings.HasPrefix(name, ".")

	displayName := name
	if isSymlink && linkTarget != "" && !opts.noSymlink && !opts.derefLinks {
		displayName += " -> " + linkTarget
	}

	var styledName string
	switch {
	case isDir:
		styledName = dirStyle.Render(displayName)
	case hidden:
		styledName = hiddenStyle.Render(displayName)
	case isSymlink:
		styledName = linkStyle.Render(displayName)
	default:
		styledName = fileStyle.Render(displayName)
	}

	if isRoot {
		fmt.Println(styledName)
		return
	}

	if opts.longFormat {
		perm := info.Mode().String()
		permCol := permStyle.Render(perm)

		size := info.Size()
		sizeStr := fmt.Sprintf("%9d", size)
		sizeCol := sizeStyle.Render(sizeStr)

		modTime := info.ModTime().Format("2006-01-02 15:04")
		modTimeCol := modTimeStyle.Render(modTime)

		fmt.Printf("%s%s %s %s %s\n", prefix, styledName, permCol, sizeCol, modTimeCol)
	} else {
		fmt.Printf("%s%s\n", prefix, styledName)
	}
}

func readDir(path string, showHidden bool, reverse bool) ([]fs.FileInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	entries, err := f.Readdir(0)
	if err != nil {
		return nil, err
	}

	filtered := make([]fs.FileInfo, 0, len(entries))
	for _, e := range entries {
		if !showHidden && strings.HasPrefix(e.Name(), ".") {
			continue
		}
		filtered = append(filtered, e)
	}

	less := func(i, j int) bool {
		if reverse {
			return strings.ToLower(filtered[i].Name()) > strings.ToLower(filtered[j].Name())
		}
		return strings.ToLower(filtered[i].Name()) < strings.ToLower(filtered[j].Name())
	}

	quickSort(filtered, 0, len(filtered)-1, less)

	return filtered, nil
}

func quickSort(entries []fs.FileInfo, low, high int, less func(i, j int) bool) {
	if low < high {
		p := partition(entries, low, high, less)
		quickSort(entries, low, p-1, less)
		quickSort(entries, p+1, high, less)
	}
}

func partition(entries []fs.FileInfo, low, high int, less func(i, j int) bool) int {
	i := low
	for j := low; j < high; j++ {
		if less(j, high) {
			entries[i], entries[j] = entries[j], entries[i]
			i++
		}
	}
	entries[i], entries[high] = entries[high], entries[i]
	return i
}
