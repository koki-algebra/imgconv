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

	"github.com/sourcegraph/conc/panics"
	"github.com/sourcegraph/conc/pool"
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

	pool := pool.New().WithErrors().WithContext(ctx)

	for _, file := range files {
		file := file
		pool.Go(func(ctx context.Context) (rerr error) {
			var c panics.Catcher
			defer func() {
				if r := c.Recovered(); r != nil {
					rerr = r.AsError()
				}
			}()

			c.Try(func() {
				rerr = convert(ctx, file)
			})

			return
		})
	}

	return pool.Wait()
}

func convert(ctx context.Context, file string) (rerr error) {
	defer trace.StartRegion(ctx, "convert "+file).End()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

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
