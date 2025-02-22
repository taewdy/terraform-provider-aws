package datasync

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/datasync"
	"github.com/hashicorp/aws-sdk-go-base/v2/awsv1shim/v2/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/sdkdiag"
	"github.com/hashicorp/terraform-provider-aws/internal/flex"
	tftags "github.com/hashicorp/terraform-provider-aws/internal/tags"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	"github.com/hashicorp/terraform-provider-aws/internal/verify"
)

// @SDKResource("aws_datasync_location_fsx_openzfs_file_system")
func ResourceLocationFSxOpenZFSFileSystem() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceLocationFSxOpenZFSFileSystemCreate,
		ReadWithoutTimeout:   resourceLocationFSxOpenZFSFileSystemRead,
		UpdateWithoutTimeout: resourceLocationFSxOpenZFSFileSystemUpdate,
		DeleteWithoutTimeout: resourceLocationFSxOpenZFSFileSystemDelete,
		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				idParts := strings.Split(d.Id(), "#")
				if len(idParts) != 2 || idParts[0] == "" || idParts[1] == "" {
					return nil, fmt.Errorf("Unexpected format of ID (%q), expected DataSyncLocationArn#FsxArn", d.Id())
				}

				DSArn := idParts[0]
				FSxArn := idParts[1]

				d.Set("fsx_filesystem_arn", FSxArn)
				d.SetId(DSArn)

				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: map[string]*schema.Schema{
			"arn": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"fsx_filesystem_arn": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: verify.ValidARN,
			},
			"protocol": {
				Type:     schema.TypeList,
				Required: true,
				ForceNew: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nfs": {
							Type:     schema.TypeList,
							Required: true,
							ForceNew: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"mount_options": {
										Type:     schema.TypeList,
										Required: true,
										ForceNew: true,
										MaxItems: 1,
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"version": {
													Type:         schema.TypeString,
													Default:      datasync.NfsVersionAutomatic,
													Optional:     true,
													ForceNew:     true,
													ValidateFunc: validation.StringInSlice(datasync.NfsVersion_Values(), false),
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"security_group_arns": {
				Type:     schema.TypeSet,
				Required: true,
				ForceNew: true,
				MinItems: 1,
				MaxItems: 5,
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: verify.ValidARN,
				},
			},
			"subdirectory": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringLenBetween(1, 4096),
			},
			"tags":     tftags.TagsSchema(),
			"tags_all": tftags.TagsSchemaComputed(),
			"uri": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"creation_time": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},

		CustomizeDiff: verify.SetTagsDiff,
	}
}

func resourceLocationFSxOpenZFSFileSystemCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).DataSyncConn()
	defaultTagsConfig := meta.(*conns.AWSClient).DefaultTagsConfig
	tags := defaultTagsConfig.MergeTags(tftags.New(ctx, d.Get("tags").(map[string]interface{})))
	fsxArn := d.Get("fsx_filesystem_arn").(string)

	input := &datasync.CreateLocationFsxOpenZfsInput{
		FsxFilesystemArn:  aws.String(fsxArn),
		Protocol:          expandProtocol(d.Get("protocol").([]interface{})),
		SecurityGroupArns: flex.ExpandStringSet(d.Get("security_group_arns").(*schema.Set)),
		Tags:              Tags(tags.IgnoreAWS()),
	}

	if v, ok := d.GetOk("subdirectory"); ok {
		input.Subdirectory = aws.String(v.(string))
	}

	log.Printf("[DEBUG] Creating DataSync Location Fsx OpenZfs File System: %#v", input)
	output, err := conn.CreateLocationFsxOpenZfsWithContext(ctx, input)
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "creating DataSync Location Fsx OpenZfs File System: %s", err)
	}

	d.SetId(aws.StringValue(output.LocationArn))

	return append(diags, resourceLocationFSxOpenZFSFileSystemRead(ctx, d, meta)...)
}

func resourceLocationFSxOpenZFSFileSystemRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).DataSyncConn()
	defaultTagsConfig := meta.(*conns.AWSClient).DefaultTagsConfig
	ignoreTagsConfig := meta.(*conns.AWSClient).IgnoreTagsConfig

	output, err := FindFSxOpenZFSLocationByARN(ctx, conn, d.Id())

	if !d.IsNewResource() && tfresource.NotFound(err) {
		log.Printf("[WARN] DataSync Location Fsx OpenZfs (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading DataSync Location Fsx OpenZfs (%s): %s", d.Id(), err)
	}

	subdirectory, err := SubdirectoryFromLocationURI(aws.StringValue(output.LocationUri))

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading DataSync Location Fsx OpenZfs (%s): %s", d.Id(), err)
	}

	d.Set("arn", output.LocationArn)
	d.Set("subdirectory", subdirectory)
	d.Set("uri", output.LocationUri)

	if err := d.Set("security_group_arns", flex.FlattenStringSet(output.SecurityGroupArns)); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting security_group_arns: %s", err)
	}

	if err := d.Set("creation_time", output.CreationTime.Format(time.RFC3339)); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting creation_time: %s", err)
	}

	if err := d.Set("protocol", flattenProtocol(output.Protocol)); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting protocol: %s", err)
	}

	tags, err := ListTags(ctx, conn, d.Id())

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "listing tags for DataSync Location Fsx OpenZfs (%s): %s", d.Id(), err)
	}

	tags = tags.IgnoreAWS().IgnoreConfig(ignoreTagsConfig)

	//lintignore:AWSR002
	if err := d.Set("tags", tags.RemoveDefaultConfig(defaultTagsConfig).Map()); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting tags: %s", err)
	}

	if err := d.Set("tags_all", tags.Map()); err != nil {
		return sdkdiag.AppendErrorf(diags, "setting tags_all: %s", err)
	}

	return diags
}

func resourceLocationFSxOpenZFSFileSystemUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).DataSyncConn()

	if d.HasChange("tags_all") {
		o, n := d.GetChange("tags_all")

		if err := UpdateTags(ctx, conn, d.Id(), o, n); err != nil {
			return sdkdiag.AppendErrorf(diags, "updating DataSync Location Fsx OpenZfs File System (%s) tags: %s", d.Id(), err)
		}
	}

	return append(diags, resourceLocationFSxOpenZFSFileSystemRead(ctx, d, meta)...)
}

func resourceLocationFSxOpenZFSFileSystemDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).DataSyncConn()

	input := &datasync.DeleteLocationInput{
		LocationArn: aws.String(d.Id()),
	}

	log.Printf("[DEBUG] Deleting DataSync Location Fsx OpenZfs File System: %#v", input)
	_, err := conn.DeleteLocationWithContext(ctx, input)

	if tfawserr.ErrMessageContains(err, datasync.ErrCodeInvalidRequestException, "not found") {
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting DataSync Location Fsx OpenZfs (%s): %s", d.Id(), err)
	}

	return diags
}

func expandProtocol(l []interface{}) *datasync.FsxProtocol {
	if len(l) == 0 || l[0] == nil {
		return nil
	}

	m := l[0].(map[string]interface{})

	Protocol := &datasync.FsxProtocol{
		NFS: expandNFS(m["nfs"].([]interface{})),
	}

	return Protocol
}

func flattenProtocol(protocol *datasync.FsxProtocol) []interface{} {
	if protocol == nil {
		return []interface{}{}
	}

	m := map[string]interface{}{
		"nfs": flattenNFS(protocol.NFS),
	}

	return []interface{}{m}
}

func expandNFS(l []interface{}) *datasync.FsxProtocolNfs {
	if len(l) == 0 || l[0] == nil {
		return nil
	}

	m := l[0].(map[string]interface{})

	Protocol := &datasync.FsxProtocolNfs{
		MountOptions: expandNFSMountOptions(m["mount_options"].([]interface{})),
	}

	return Protocol
}

func flattenNFS(nfs *datasync.FsxProtocolNfs) []interface{} {
	if nfs == nil {
		return []interface{}{}
	}

	m := map[string]interface{}{
		"mount_options": flattenNFSMountOptions(nfs.MountOptions),
	}

	return []interface{}{m}
}
