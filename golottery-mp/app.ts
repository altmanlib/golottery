App({
  globalData: {
    eventPublicId: "" as string,
  },
  onLaunch(options: WechatMiniprogram.App.LaunchShowOption) {
    const q = options.query ?? {}
    if (typeof q.e === "string" && q.e) {
      this.globalData.eventPublicId = q.e
    }
  },
})
