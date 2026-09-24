const app = getApp<IAppOption>()

Page({
  data: {
    eventPublicId: "",
  },
  onShow() {
    this.setData({
      eventPublicId: app.globalData.eventPublicId || "",
    })
  },
})
