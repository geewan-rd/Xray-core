package inbound

import (
	context "context"
	"fmt"

	protocol "github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/uuid"
	"github.com/xtls/xray-core/proxy/vless"
)

type API struct {
	UnimplementedVLESSAPIServer
	validator *vless.MemoryValidator
}

func NewApi(v *vless.MemoryValidator) *API {
	return &API{
		validator: v,
	}
}

func (a *API) AddUser(ctx context.Context, req *AddUserRequest) (*AddUserResponse, error) {
	if req.Id == "" {
		return &AddUserResponse{Error: "ID is empty"}, nil
	}
	id, err := uuid.ParseString(req.Id)
	if err != nil {
		return &AddUserResponse{Error: fmt.Sprintf("invalid UUID format: %v", err)}, nil
	}
	account := &vless.MemoryAccount{
		ID:         protocol.NewID(id),
		Flow:       "xtls-rprx-vision",
		Encryption: "none",
	}
	err = a.validator.Add(&protocol.MemoryUser{
		Account: account,
	})
	if err != nil {
		return &AddUserResponse{Error: fmt.Sprintf("failed to add user: %v", err)}, nil
	}

	return &AddUserResponse{}, nil
}
func (a *API) RemoveUser(ctx context.Context, req *RemoveUserRequest) (*RemoveUserResponse, error) {
	if req.Id == "" {
		return &RemoveUserResponse{Error: "ID is empty"}, nil
	}
	id, err := uuid.ParseString(req.Id)
	if err != nil {
		return &RemoveUserResponse{Error: fmt.Sprintf("invalid UUID format: %v", err)}, nil
	}
	err = a.validator.DelByID(id)
	if err != nil {
		return &RemoveUserResponse{Error: fmt.Sprintf("failed to remove user: %v", err)}, nil
	}

	return &RemoveUserResponse{}, nil
}
func (a *API) GetUser(ctx context.Context, req *GetUserRequest) (*GetUserResponse, error) {
	return &GetUserResponse{Id: req.Id}, nil
}
func (a *API) GetUsers(context.Context, *GetUsersRequest) (*GetUsersResponse, error) {
	users := a.validator.GetAllIDs()
	resp := &GetUsersResponse{
		Ids: make([]string, 0, len(users)),
	}
	for _, u := range users {
		resp.Ids = append(resp.Ids, u.Account.(*vless.MemoryAccount).ID.String())
	}
	return resp, nil
}
