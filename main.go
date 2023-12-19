package main

import (
	"context"
	"errors"
	"fmt"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"runtime/trace"
	"sync"
)

func main() {
	ctx := context.Background()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, files []string) error {
	file, err := os.Create("trace.out")
	if err != nil {
		return err
	}
	defer file.Close()

	if err := trace.Start(file); err != nil {
		return err
	}

	if err := convertAll(ctx, files); err != nil {
		return err
	}

	trace.Stop()

	if err := file.Sync(); err != nil {
		return err
	}

	return nil
}

func convertAll(ctx context.Context, files []string) error {
	ctx, task := trace.NewTask(ctx, "convert all")
	defer task.End()

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		rerr error
	)

	for _, file := range files {
		file := file
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := convert(ctx, file); err != nil {
				mu.Lock()
				if rerr == nil {
					rerr = err
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	return nil
}

func convert(ctx context.Context, file string) (rerr error) {
	defer trace.StartRegion(ctx, "convert "+file).End()

	src, err := os.Open(file)
	if err != nil {
		return err
	}
	defer src.Close()

	pngimg, err := png.Decode(src)
	if err != nil {
		return err
	}

	ext := filepath.Ext(file)
	jpgfile := file[:len(file)-len(ext)] + ".jpg"

	dst, err := os.Create(jpgfile)
	if err != nil {
		return err
	}
	defer func() {
		dst.Close()
		if rerr != nil {
			rerr = errors.Join(rerr, os.Remove(jpgfile))
		}
	}()

	if err := jpeg.Encode(dst, pngimg, nil); err != nil {
		return err
	}

	if err := dst.Sync(); err != nil {
		return err
	}

	return nil
}
