package userinfo

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	kyvernov2 "github.com/kyverno/kyverno/api/kyverno/v2"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/command"
	"github.com/kyverno/kyverno/cmd/cli/kubectl-kyverno/commands/create/templates"
	"github.com/spf13/cobra"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/util/validation"
)

type options struct {
	Name string
	kyvernov2.RequestInfo
}

func Command() *cobra.Command {
	return commandWithTemplate(templates.UserInfoTemplate)
}

func commandWithTemplate(userinfoTemplate string) *cobra.Command {
	var path string
	var name string
	var username string
	var roles, clusterRoles, groups []string
	cmd := &cobra.Command{
		Use:          "user-info",
		Short:        command.FormatDescription(true, websiteUrl, false, description...),
		Long:         command.FormatDescription(false, websiteUrl, false, description...),
		Example:      command.FormatExamples(examples...),
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if errs := validation.IsDNS1123Subdomain(name); len(errs) > 0 {
				return fmt.Errorf("invalid name %q: %s", name, strings.Join(errs, "; "))
			}
			tmpl, err := template.New("userinfo").Parse(userinfoTemplate)
			if err != nil {
				return err
			}
			values := options{
				Name: name,
				RequestInfo: kyvernov2.RequestInfo{
					Roles:        roles,
					ClusterRoles: clusterRoles,
					AdmissionUserInfo: authenticationv1.UserInfo{
						Username: username,
						Groups:   groups,
					},
				},
			}
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, values); err != nil {
				return err
			}
			if path != "" {
				return os.WriteFile(path, buf.Bytes(), 0o644)
			}
			_, err = cmd.OutOrStdout().Write(buf.Bytes())
			return err
		},
	}
	cmd.Flags().StringVarP(&path, "output", "o", "", "Output path (uses standard console output if not set)")
	cmd.Flags().StringVarP(&name, "name", "n", "user-info", "User info name")
	cmd.Flags().StringVarP(&username, "username", "u", "", "User name")
	cmd.Flags().StringSliceVarP(&roles, "role", "r", nil, "Role")
	cmd.Flags().StringSliceVarP(&clusterRoles, "cluster-role", "c", nil, "Cluster role")
	cmd.Flags().StringSliceVarP(&groups, "group", "g", nil, "Group")
	return cmd
}
