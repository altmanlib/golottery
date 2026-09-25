package apihttp

import (
	"bytes"
	"context"
	"errors"
	"net/url"

	api "golottery/api/api"
	"golottery/api/internal/bizerr"
	"golottery/api/internal/event"
	"golottery/api/internal/wechat"
)

// GetEventQRCode returns the mini program code of an event. The scene is the public id;
// the mini program reads it from decodeURIComponent(query.scene).
func (s *Server) GetEventQRCode(ctx context.Context, request api.GetEventQRCodeRequestObject) (api.GetEventQRCodeResponseObject, error) {
	admin, err := adminFrom(ctx)
	if err != nil {
		return nil, err
	}
	view, err := s.events.Get(ctx, admin.OrgID, request.EventId)
	if err != nil {
		return nil, err
	}
	if s.wechat == nil || !s.wechat.Configured() {
		return nil, bizerr.New(bizerr.CodeWechatNotConfigured)
	}
	img, err := s.wechat.UnlimitedQRCode(ctx, view.PublicID, event.EntryPage)
	if errors.Is(err, wechat.ErrNotConfigured) {
		return nil, bizerr.New(bizerr.CodeWechatNotConfigured)
	}
	if err != nil {
		// The WeChat errcode stays in the cause, which recoverBizErr logs.
		return nil, bizerr.Wrap(bizerr.CodeInternal, err)
	}
	disposition := `attachment; filename="qrcode.png"; filename*=UTF-8''` + url.PathEscape(view.Name+"-小程序码.png")
	return api.GetEventQRCode200ImagepngResponse{
		Body:          bytes.NewReader(img),
		ContentLength: int64(len(img)),
		Headers:       api.GetEventQRCode200ResponseHeaders{ContentDisposition: &disposition},
	}, nil
}
