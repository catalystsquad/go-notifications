package notifo_store

import (
	"encoding/json"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func GetStructFromProtoWithMarshaller(marshaller protojson.MarshalOptions, source proto.Message, dest interface{}) error {
	bytes, err := marshaller.Marshal(source)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, &dest)
}

func GetProtoFromStructWithMarshaller(unmarshaller protojson.UnmarshalOptions, source interface{}, dest proto.Message) error {
	bytes, err := json.Marshal(source)
	if err != nil {
		return err
	}
	return unmarshaller.Unmarshal(bytes, dest)
}
