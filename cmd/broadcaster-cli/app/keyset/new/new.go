package new

import (
	"bufio"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/bitcoin-sv/arc/internal/keyset"
	"github.com/lmittmann/tint"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var Cmd = &cobra.Command{
	Use:   "new",
	Short: "Create new key set",
	RunE: func(cmd *cobra.Command, args []string) error {

		keyFile := viper.GetString("filename")

		newKeyset, err := keyset.New()
		if err != nil {
			return err
		}

		// if keyfile not given, create new file name with iterator
		if keyFile == "" {
			i := 0
			for {
				keyFile = fmt.Sprintf("./cmd/broadcaster-cli/arc-%d.key", i)
				_, err = os.Open(keyFile)
				if os.IsNotExist(err) {
					break
				}
				i++
			}
		}

		// return error if file already exists -> do not overwrite key files
		_, err = os.Open(keyFile)
		if !os.IsNotExist(err) {
			return fmt.Errorf("key file %s already exists", keyFile)
		}

		writer, err := os.Create(keyFile)
		if err != nil {
			return err
		}

		defer writer.Close()

		buffer := bufio.NewWriter(writer)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(buffer, newKeyset.GetMaster().String())
		if err != nil {
			return err
		}

		err = buffer.Flush()
		if err != nil {
			return err
		}

		logger := slog.New(tint.NewHandler(os.Stdout, &tint.Options{Level: slog.LevelInfo}))
		logger.Info("new key file created", slog.String("file", keyFile))
		return nil
	},
}

func init() {
	var err error

	Cmd.Flags().String("filename", "", "Name of new key file")
	err = viper.BindPFlag("filename", Cmd.Flags().Lookup("filename"))
	if err != nil {
		log.Fatal(err)
	}

}
