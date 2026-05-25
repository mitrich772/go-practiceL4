// mygrep — клиент распределённого grep.
//
// Читает stdin или файл, распределяет данные между серверами и выводит
// итог, как делал бы обычный grep. Поддерживаемые флаги перечислены в
// `mygrep -h`.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"mygrep/internal/client"
	"mygrep/internal/protocol"
)

func main() {
	pattern := flag.String("e", "", "pattern to search for (alias: --pattern)")
	patternLong := flag.String("pattern", "", "pattern to search for")
	ignoreCase := flag.Bool("i", false, "ignore case")
	invertMatch := flag.Bool("v", false, "invert match")
	fixedString := flag.Bool("F", false, "treat pattern as a fixed string (no regex)")
	printLineNum := flag.Bool("n", false, "print line numbers (1-based, global)")
	countOnly := flag.Bool("c", false, "print only count of matching lines")
	listFiles := flag.Bool("l", false, "print only file names that contain matches")
	serversFlag := flag.String("servers", "", "comma-separated list of server addresses host:port (required)")
	quorumFlag := flag.Int("quorum", 0, "minimum number of servers that must respond successfully (default N/2+1)")
	timeoutSec := flag.Int("timeout", 30, "request timeout in seconds")
	connectRetrySec := flag.Int("connect-retry", 5, "how long (seconds) to keep retrying transient dial errors (DNS / connection refused) before giving up; useful when servers are still starting (e.g. docker compose)")
	flag.Usage = usage
	os.Args = append([]string{os.Args[0]}, expandShortFlags(os.Args[1:])...)
	flag.Parse()

	pat := *pattern
	if pat == "" {
		pat = *patternLong
	}
	if pat == "" {
		exitWith(2, "error: pattern is required (use -e or --pattern)\n")
	}
	if *serversFlag == "" {
		exitWith(2, "error: --servers is required (comma-separated host:port list)\n")
	}

	var (
		input    string
		fileName string
		err      error
	)
	if flag.NArg() > 0 {
		fileName = flag.Arg(0)
		input, err = readFile(fileName)
		if err != nil {
			exitWith(2, "error: %v\n", err)
		}
	} else {
		input, err = readAll(os.Stdin)
		if err != nil {
			exitWith(2, "error reading stdin: %v\n", err)
		}
	}

	serverList := splitAndTrim(*serversFlag, ",")
	if len(serverList) == 0 {
		exitWith(2, "error: --servers must contain at least one address\n")
	}

	flags := protocol.GrepFlags{
		Pattern:      pat,
		IgnoreCase:   *ignoreCase,
		InvertMatch:  *invertMatch,
		FixedString:  *fixedString,
		PrintLineNum: *printLineNum,
		CountOnly:    *countOnly,
		ListFiles:    *listFiles,
	}

	cfg := client.Config{
		Servers:      serverList,
		Quorum:       *quorumFlag,
		Timeout:      time.Duration(*timeoutSec) * time.Second,
		ConnectRetry: time.Duration(*connectRetrySec) * time.Second,
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout+5*time.Second)
	defer cancel()

	res, err := client.Run(ctx, cfg, flags, fileName, input)
	if err != nil {
		exitWith(2, "error: %v\n", err)
	}
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	switch {
	case flags.CountOnly:
		fmt.Fprintln(out, res.Count)
	case flags.ListFiles:
		if res.HasMatch && fileName != "" {
			fmt.Fprintln(out, fileName)
		}
	default:
		for _, m := range res.Matches {
			if flags.PrintLineNum {
				fmt.Fprintf(out, "%d:%s\n", m.LineNum, m.Text)
			} else {
				fmt.Fprintln(out, m.Text)
			}
		}
	}
	_ = out.Flush()

	if !res.QuorumMet {
		fmt.Fprintf(os.Stderr,
			"warning: quorum not reached (%d/%d, requested %d)\n",
			res.Successes, res.Servers, res.Quorum)
		for _, e := range res.ErrorsList {
			fmt.Fprintln(os.Stderr, "  -", e)
		}
		// Exit code 2 — поведение, совместимое с grep при ошибке.
		os.Exit(2)
	}

	// grep возвращает 0, если совпадение найдено, и 1, если не найдено.
	if flags.CountOnly {
		if res.Count == 0 {
			os.Exit(1)
		}
		return
	}
	if !res.HasMatch {
		os.Exit(1)
	}
}

func usage() {
	out := flag.CommandLine.Output()
	fmt.Fprintln(out, "mygrep — распределённый grep с кворумом")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  mygrep --servers host1:port,host2:port,... -e PATTERN [flags] [file]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Если file не указан, вход читается из stdin.")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Flags:")
	flag.PrintDefaults()
}

func readFile(name string) (string, error) {
	f, err := os.Open(name)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", name, err)
	}
	defer f.Close()
	return readAll(f)
}

// readAll читает поток целиком. Не используем bufio.Scanner, чтобы не
// упереться в дефолтный лимит длины строки.
func readAll(r io.Reader) (string, error) {
	var sb strings.Builder
	buf := make([]byte, 64*1024)
	for {
		nRead, err := r.Read(buf)
		if nRead > 0 {
			sb.Write(buf[:nRead])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return sb.String(), nil
}

func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func exitWith(code int, format string, args ...any) {
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(code)
}

// expandShortFlags разворачивает группы коротких булевых флагов
// (-Fni → -F -n -i), как это делает GNU grep. Затрагиваются только аргументы,
// у которых все символы — известные булевые короткие флаги; всё прочее
// (-e PATTERN, -pattern=..., --servers=...) проходит без изменений.
func expandShortFlags(args []string) []string {
	boolFlags := map[byte]bool{
		'i': true, 'v': true, 'F': true,
		'n': true, 'c': true, 'l': true,
	}
	out := make([]string, 0, len(args))
	for _, a := range args {
		if len(a) < 3 || a[0] != '-' || a[1] == '-' {
			out = append(out, a)
			continue
		}
		// Проверяем, что все буквы — булевы короткие флаги.
		ok := true
		for i := 1; i < len(a); i++ {
			if !boolFlags[a[i]] {
				ok = false
				break
			}
		}
		if !ok {
			out = append(out, a)
			continue
		}
		for i := 1; i < len(a); i++ {
			out = append(out, "-"+string(a[i]))
		}
	}
	return out
}
