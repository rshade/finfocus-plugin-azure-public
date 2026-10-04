package pricing

import (
	"fmt"
	"maps"
	"sort"
	"strconv"
	"strings"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	finfocusv1 "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

// withAttributeTags returns desc with ResourceDescriptor.attributes merged into
// its tags, so every tag reader sees the structured inputs. A descriptor with no
// attributes is returned unchanged. The merged copy keeps tag-only keys, and an
// attribute value replaces a tag with the same key, as the field contract says.
// The request descriptor is never modified.
//
// attributes may carry user data; it is never logged.
func withAttributeTags(desc *finfocusv1.ResourceDescriptor) (*finfocusv1.ResourceDescriptor, error) {
	attrs := desc.GetAttributes()
	if len(attrs.GetFields()) == 0 {
		return desc, nil
	}
	if size := proto.Size(attrs); size > pluginsdk.MaxAttributesBytes {
		return nil, status.Errorf(codes.InvalidArgument,
			"resource attributes are %d bytes, over the %d byte limit", size, pluginsdk.MaxAttributesBytes)
	}

	merged, ok := proto.Clone(desc).(*finfocusv1.ResourceDescriptor)
	if !ok {
		return nil, status.Error(codes.Internal, "clone resource descriptor")
	}
	merged.Attributes = nil
	tags := make(map[string]string, len(desc.GetTags())+len(attrs.GetFields()))
	maps.Copy(tags, desc.GetTags())
	maps.Copy(tags, attributeTags(attrs))
	merged.Tags = tags
	return merged, nil
}

// Bounds on the tags built from attributes. Core caps dotted keys at 6
// segments and 50 tags; attributes carry the whole structure, so these are
// far higher but finite. The deepest key the plugin reads has 4 segments.
const (
	maxAttributeDepth  = 32
	maxAttributeTags   = 4096
	maxAttributeKeyLen = 256
)

type attributeNode struct {
	path  string
	depth int
	value any
}

// attributeTags flattens attributes the way finfocus core's ConvertToProto
// flattens Pulumi inputs into tags (internal/engine/flatten.go): every
// top-level key keeps its collapsed value, and nested scalars add dotted keys
// such as sku.capacity or zones.0. Like core's PrepareProjectedDescriptor, a
// value whose text contains the Pulumi unknown placeholder is dropped, so the
// tag of the same name still applies. Dotted keys are added shallowest first
// until maxAttributeTags, and stop at maxAttributeDepth segments or
// maxAttributeKeyLen bytes.
func attributeTags(attrs *structpb.Struct) map[string]string {
	properties := attrs.AsMap()
	keys := sortedKeys(properties)
	tags := make(map[string]string, len(keys))
	queue := make([]attributeNode, 0, len(keys))
	for _, key := range keys {
		if len(tags) < maxAttributeTags {
			addAttributeTag(tags, key, properties[key])
		}
		queue = append(queue, attributeNode{path: key, depth: 1, value: properties[key]})
	}
	for len(queue) > 0 && len(tags) < maxAttributeTags {
		node := queue[0]
		queue = expandAttributeNode(tags, queue[1:], node)
	}
	return tags
}

// expandAttributeNode adds node's tag when it is a nested scalar, or queues its
// children when it is an object or list that is neither redacted nor a user tag
// map.
func expandAttributeNode(tags map[string]string, queue []attributeNode, node attributeNode) []attributeNode {
	segment := node.path[strings.LastIndex(node.path, ".")+1:]
	if skipAttributeSegment(segment) {
		return queue
	}
	switch typed := node.value.(type) {
	case map[string]any:
		if skipAttributeContainer(segment) {
			return queue
		}
		for _, key := range sortedKeys(typed) {
			queue = appendAttributeChild(queue, node, key, typed[key])
		}
	case []any:
		if skipAttributeContainer(segment) {
			return queue
		}
		for i, elem := range typed {
			queue = appendAttributeChild(queue, node, strconv.Itoa(i), elem)
		}
	default:
		if node.depth > 1 && node.value != nil {
			addAttributeTag(tags, node.path, node.value)
		}
	}
	return queue
}

func appendAttributeChild(queue []attributeNode, parent attributeNode, segment string, value any) []attributeNode {
	path := parent.path + "." + segment
	if parent.depth+1 > maxAttributeDepth || len(path) > maxAttributeKeyLen {
		return queue
	}
	return append(queue, attributeNode{path: path, depth: parent.depth + 1, value: value})
}

func addAttributeTag(tags map[string]string, key string, value any) {
	if text := collapsedTagValue(value); !strings.Contains(text, pulumiUnknownValue) {
		tags[key] = text
	}
}

// collapsedTagValue is core's ConvertValueToString: whole-number floats print
// without a fraction, an object collapses to its value, id, or name (or its only
// field), and a list joins its elements with commas.
func collapsedTagValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(typed)
	case map[string]any:
		for _, key := range []string{"value", "id", "name"} {
			if inner, ok := typed[key]; ok {
				return collapsedTagValue(inner)
			}
		}
		if len(typed) == 1 {
			for _, inner := range typed {
				return collapsedTagValue(inner)
			}
		}
		return fmt.Sprintf("%v", typed)
	case []any:
		parts := make([]string, len(typed))
		for i, elem := range typed {
			parts[i] = collapsedTagValue(elem)
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprintf("%v", typed)
	}
}

// skipAttributeSegment mirrors core's skipDottedSegment. Hosts already redact
// these keys from attributes; the check keeps them out of tags if one does not.
func skipAttributeSegment(segment string) bool {
	if strings.HasPrefix(segment, "__") {
		return true
	}
	lower := strings.ToLower(segment)
	for _, fragment := range []string{
		"password", "secret", "token", "credential", "ciphertext", "privatekey",
		"apikey", "api_key", "accesskey", "access_key", "connectionstring", "connection_string",
	} {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}

// skipAttributeContainer mirrors core's skipFlattenContainer: user tag maps are
// not expanded into dotted keys.
func skipAttributeContainer(segment string) bool {
	switch segment {
	case "tags", "tagsAll", "labels", "annotations":
		return true
	default:
		return false
	}
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
