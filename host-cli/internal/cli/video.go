package cli

import (
	"github.com/pkronstrom/svalbard/host-cli/internal/commands"
	"github.com/spf13/cobra"
)

func addVideoOptions(cmd *cobra.Command) {
	cmd.Flags().String("video-index", "off", "ZIM video search: off, captions, or transcribe (uses Docker)")
	cmd.Flags().String("video-zim", "", "limit optional video indexing to this installed .zim filename")
	cmd.Flags().Int("video-limit", 0, "maximum new video transcriptions this run (0 for all)")
}

func videoOptions(cmd *cobra.Command) (commands.VideoIndexOptions, error) {
	mode, _ := cmd.Flags().GetString("video-index")
	archive, _ := cmd.Flags().GetString("video-zim")
	limit, _ := cmd.Flags().GetInt("video-limit")
	options := commands.VideoIndexOptions{Mode: mode, Archive: archive, Limit: limit}
	return options, options.Validate()
}
