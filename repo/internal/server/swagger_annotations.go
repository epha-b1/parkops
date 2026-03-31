package server

// @Summary List facilities
// @Tags Master Data
// @Security SessionCookie
// @Produce json
// @Success 200 {array} map[string]interface{}
// @Router /facilities [get]
func swaggerFacilitiesList() {}

// @Summary Create facility
// @Tags Master Data
// @Security SessionCookie
// @Accept json
// @Produce json
// @Success 201 {object} map[string]interface{}
// @Router /facilities [post]
func swaggerFacilitiesCreate() {}

// @Summary Get facility
// @Tags Master Data
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /facilities/{id} [get]
func swaggerFacilitiesGet() {}

// @Summary Update facility
// @Tags Master Data
// @Security SessionCookie
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /facilities/{id} [patch]
func swaggerFacilitiesPatch() {}

// @Summary Delete facility
// @Tags Master Data
// @Security SessionCookie
// @Success 204
// @Router /facilities/{id} [delete]
func swaggerFacilitiesDelete() {}

// @Summary List lots
// @Tags Master Data
// @Security SessionCookie
// @Produce json
// @Success 200 {array} map[string]interface{}
// @Router /lots [get]
func swaggerLotsList() {}

// @Summary Create lot
// @Tags Master Data
// @Security SessionCookie
// @Accept json
// @Produce json
// @Success 201 {object} map[string]interface{}
// @Router /lots [post]
func swaggerLotsCreate() {}

// @Summary Get lot
// @Tags Master Data
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /lots/{id} [get]
func swaggerLotsGet() {}

// @Summary Update lot
// @Tags Master Data
// @Security SessionCookie
// @Accept json
// @Success 200 {object} map[string]interface{}
// @Router /lots/{id} [patch]
func swaggerLotsPatch() {}

// @Summary Delete lot
// @Tags Master Data
// @Security SessionCookie
// @Success 204
// @Router /lots/{id} [delete]
func swaggerLotsDelete() {}

// @Summary List zones
// @Tags Master Data
// @Security SessionCookie
// @Produce json
// @Success 200 {array} map[string]interface{}
// @Router /zones [get]
func swaggerZonesList() {}

// @Summary Create zone
// @Tags Master Data
// @Security SessionCookie
// @Accept json
// @Produce json
// @Success 201 {object} map[string]interface{}
// @Router /zones [post]
func swaggerZonesCreate() {}

// @Summary Get zone
// @Tags Master Data
// @Security SessionCookie
// @Success 200 {object} map[string]interface{}
// @Router /zones/{id} [get]
func swaggerZonesGet() {}

// @Summary Update zone
// @Tags Master Data
// @Security SessionCookie
// @Accept json
// @Success 200 {object} map[string]interface{}
// @Router /zones/{id} [patch]
func swaggerZonesPatch() {}

// @Summary Delete zone
// @Tags Master Data
// @Security SessionCookie
// @Success 204
// @Router /zones/{id} [delete]
func swaggerZonesDelete() {}

// @Summary List rate plans
// @Tags Master Data
// @Security SessionCookie
// @Success 200 {array} map[string]interface{}
// @Router /rate-plans [get]
func swaggerRatePlansList() {}

// @Summary Create rate plan
// @Tags Master Data
// @Security SessionCookie
// @Accept json
// @Success 201 {object} map[string]interface{}
// @Router /rate-plans [post]
func swaggerRatePlansCreate() {}

// @Summary Get rate plan
// @Tags Master Data
// @Security SessionCookie
// @Success 200 {object} map[string]interface{}
// @Router /rate-plans/{id} [get]
func swaggerRatePlansGet() {}

// @Summary Update rate plan
// @Tags Master Data
// @Security SessionCookie
// @Accept json
// @Success 200 {object} map[string]interface{}
// @Router /rate-plans/{id} [patch]
func swaggerRatePlansPatch() {}

// @Summary Delete rate plan
// @Tags Master Data
// @Security SessionCookie
// @Success 204
// @Router /rate-plans/{id} [delete]
func swaggerRatePlansDelete() {}

// @Summary List members
// @Tags Master Data
// @Security SessionCookie
// @Success 200 {object} map[string]interface{}
// @Router /members [get]
func swaggerMembersList() {}

// @Summary Create member
// @Tags Master Data
// @Security SessionCookie
// @Accept json
// @Success 201 {object} map[string]interface{}
// @Router /members [post]
func swaggerMembersCreate() {}

// @Summary Get member
// @Tags Master Data
// @Security SessionCookie
// @Success 200 {object} map[string]interface{}
// @Router /members/{id} [get]
func swaggerMembersGet() {}

// @Summary Update member
// @Tags Master Data
// @Security SessionCookie
// @Accept json
// @Success 200 {object} map[string]interface{}
// @Router /members/{id} [patch]
func swaggerMembersPatch() {}

// @Summary Delete member
// @Tags Master Data
// @Security SessionCookie
// @Success 204
// @Router /members/{id} [delete]
func swaggerMembersDelete() {}

// @Summary Get member balance
// @Tags Master Data
// @Security SessionCookie
// @Success 200 {object} map[string]interface{}
// @Router /members/{id}/balance [get]
func swaggerMembersBalanceGet() {}

// @Summary Patch member balance
// @Tags Master Data
// @Security SessionCookie
// @Accept json
// @Success 200 {object} map[string]interface{}
// @Router /members/{id}/balance [patch]
func swaggerMembersBalancePatch() {}

// @Summary List vehicles
// @Tags Master Data
// @Security SessionCookie
// @Success 200 {array} map[string]interface{}
// @Router /vehicles [get]
func swaggerVehiclesList() {}

// @Summary Create vehicle
// @Tags Master Data
// @Security SessionCookie
// @Accept json
// @Success 201 {object} map[string]interface{}
// @Router /vehicles [post]
func swaggerVehiclesCreate() {}

// @Summary Get vehicle
// @Tags Master Data
// @Security SessionCookie
// @Success 200 {object} map[string]interface{}
// @Router /vehicles/{id} [get]
func swaggerVehiclesGet() {}

// @Summary Update vehicle
// @Tags Master Data
// @Security SessionCookie
// @Accept json
// @Success 200 {object} map[string]interface{}
// @Router /vehicles/{id} [patch]
func swaggerVehiclesPatch() {}

// @Summary Delete vehicle
// @Tags Master Data
// @Security SessionCookie
// @Success 204
// @Router /vehicles/{id} [delete]
func swaggerVehiclesDelete() {}

// @Summary List drivers
// @Tags Master Data
// @Security SessionCookie
// @Success 200 {array} map[string]interface{}
// @Router /drivers [get]
func swaggerDriversList() {}

// @Summary Create driver
// @Tags Master Data
// @Security SessionCookie
// @Accept json
// @Success 201 {object} map[string]interface{}
// @Router /drivers [post]
func swaggerDriversCreate() {}

// @Summary Get driver
// @Tags Master Data
// @Security SessionCookie
// @Success 200 {object} map[string]interface{}
// @Router /drivers/{id} [get]
func swaggerDriversGet() {}

// @Summary Update driver
// @Tags Master Data
// @Security SessionCookie
// @Accept json
// @Success 200 {object} map[string]interface{}
// @Router /drivers/{id} [patch]
func swaggerDriversPatch() {}

// @Summary Delete driver
// @Tags Master Data
// @Security SessionCookie
// @Success 204
// @Router /drivers/{id} [delete]
func swaggerDriversDelete() {}

// @Summary List message rules
// @Tags Master Data
// @Security SessionCookie
// @Success 200 {array} map[string]interface{}
// @Router /message-rules [get]
func swaggerMessageRulesList() {}

// @Summary Create message rule
// @Tags Master Data
// @Security SessionCookie
// @Accept json
// @Success 201 {object} map[string]interface{}
// @Router /message-rules [post]
func swaggerMessageRulesCreate() {}

// @Summary Update message rule
// @Tags Master Data
// @Security SessionCookie
// @Accept json
// @Success 200 {object} map[string]interface{}
// @Router /message-rules/{id} [patch]
func swaggerMessageRulesPatch() {}

// @Summary Delete message rule
// @Tags Master Data
// @Security SessionCookie
// @Success 204
// @Router /message-rules/{id} [delete]
func swaggerMessageRulesDelete() {}

// @Summary Create reservation hold
// @Tags Reservations
// @Security SessionCookie
// @Accept json
// @Produce json
// @Success 201 {object} map[string]interface{}
// @Router /reservations/hold [post]
func swaggerReservationHoldCreate() {}

// @Summary Confirm reservation hold
// @Tags Reservations
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /reservations/{id}/confirm [post]
func swaggerReservationConfirm() {}

// @Summary Cancel reservation
// @Tags Reservations
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /reservations/{id}/cancel [post]
func swaggerReservationCancel() {}

// @Summary Get availability for window
// @Tags Capacity
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /availability [get]
func swaggerAvailabilityGet() {}

// @Summary Capacity dashboard
// @Tags Capacity
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /capacity/dashboard [get]
func swaggerCapacityDashboard() {}

// @Summary Zone stalls for time window
// @Tags Capacity
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /capacity/zones/{id}/stalls [get]
func swaggerZoneStallsGet() {}

// @Summary List capacity snapshots
// @Tags Capacity
// @Security SessionCookie
// @Produce json
// @Success 200 {array} map[string]interface{}
// @Router /capacity/snapshots [get]
func swaggerCapacitySnapshotsList() {}

// @Summary Reservation stats for today
// @Tags Reservations
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /reservations/stats/today [get]
func swaggerReservationStatsToday() {}

// @Summary List reservations
// @Tags Reservations
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /reservations [get]
func swaggerReservationsList() {}

// @Summary Reservation timeline
// @Tags Reservations
// @Security SessionCookie
// @Produce json
// @Success 200 {array} map[string]interface{}
// @Router /reservations/{id}/timeline [get]
func swaggerReservationTimeline() {}

// @Summary List open exceptions
// @Tags Exceptions
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /exceptions [get]
func swaggerExceptionsList() {}

// @Summary Get exception
// @Tags Exceptions
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /exceptions/{id} [get]
func swaggerExceptionsGet() {}

// @Summary Acknowledge exception
// @Tags Exceptions
// @Security SessionCookie
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /exceptions/{id}/acknowledge [post]
func swaggerExceptionsAcknowledge() {}

// @Summary List acknowledged exceptions history
// @Tags Exceptions
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /exceptions/history [get]
func swaggerExceptionsHistory() {}

// @Summary Register device
// @Tags Devices
// @Security SessionCookie
// @Accept json
// @Produce json
// @Success 201 {object} map[string]interface{}
// @Router /devices [post]
func swaggerDeviceCreate() {}

// @Summary List devices
// @Tags Devices
// @Security SessionCookie
// @Produce json
// @Success 200 {array} map[string]interface{}
// @Router /devices [get]
func swaggerDeviceList() {}

// @Summary Get device
// @Tags Devices
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /devices/{id} [get]
func swaggerDeviceGet() {}

// @Summary Update device
// @Tags Devices
// @Security SessionCookie
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /devices/{id} [patch]
func swaggerDevicePatch() {}

// @Summary Delete device
// @Tags Devices
// @Security SessionCookie
// @Success 204
// @Router /devices/{id} [delete]
func swaggerDeviceDelete() {}

// @Summary Ingest device event
// @Tags Devices
// @Security SessionCookie
// @Accept json
// @Produce json
// @Success 201 {object} map[string]interface{}
// @Success 200 {object} map[string]interface{}
// @Router /device-events [post]
func swaggerDeviceEventCreate() {}

// @Summary List device events
// @Tags Devices
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /device-events [get]
func swaggerDeviceEventsList() {}

// @Summary Replay device events
// @Tags Devices
// @Security SessionCookie
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /device-events/replay [post]
func swaggerDeviceEventsReplay() {}

// @Summary Submit tracking location report
// @Tags Tracking
// @Security SessionCookie
// @Accept json
// @Produce json
// @Success 201 {object} map[string]interface{}
// @Router /tracking/location [post]
func swaggerTrackingLocationCreate() {}

// @Summary List vehicle positions
// @Tags Tracking
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /tracking/vehicles/{id}/positions [get]
func swaggerTrackingPositionsList() {}

// @Summary List vehicle stop events
// @Tags Tracking
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /tracking/vehicles/{id}/stops [get]
func swaggerTrackingStopsList() {}

// @Summary Run reconciliation now
// @Tags Capacity
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /reconciliation/run [post]
func swaggerReconciliationRun() {}

// @Summary List notification topics
// @Tags Notifications
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /notification-topics [get]
func swaggerNotificationTopicsList() {}

// @Summary Subscribe notification topic
// @Tags Notifications
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /notification-topics/{id}/subscribe [post]
func swaggerNotificationTopicSubscribe() {}

// @Summary Unsubscribe notification topic
// @Tags Notifications
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /notification-topics/{id}/subscribe [delete]
func swaggerNotificationTopicUnsubscribe() {}

// @Summary Get notification settings
// @Tags Notifications
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /notification-settings [get]
func swaggerNotificationSettingsGet() {}

// @Summary Patch notification settings
// @Tags Notifications
// @Security SessionCookie
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /notification-settings [patch]
func swaggerNotificationSettingsPatch() {}

// @Summary Get DND settings
// @Tags Notifications
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /notification-settings/dnd [get]
func swaggerNotificationDNDGet() {}

// @Summary Patch DND settings
// @Tags Notifications
// @Security SessionCookie
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /notification-settings/dnd [patch]
func swaggerNotificationDNDPatch() {}

// @Summary List notifications
// @Tags Notifications
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /notifications [get]
func swaggerNotificationsList() {}

// @Summary Get notification
// @Tags Notifications
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /notifications/{id} [get]
func swaggerNotificationGet() {}

// @Summary Mark notification read
// @Tags Notifications
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /notifications/{id}/read [patch]
func swaggerNotificationRead() {}

// @Summary Dismiss notification
// @Tags Notifications
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /notifications/{id}/dismiss [post]
func swaggerNotificationDismiss() {}

// @Summary List export packages
// @Tags Notifications
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /notifications/export-packages [get]
func swaggerNotificationExportsList() {}

// @Summary Download export package
// @Tags Notifications
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /notifications/export-packages/{id}/download [get]
func swaggerNotificationExportsDownload() {}

// @Summary List campaigns
// @Tags Campaigns
// @Security SessionCookie
// @Produce json
// @Success 200 {array} map[string]interface{}
// @Router /campaigns [get]
func swaggerCampaignsList() {}

// @Summary Create campaign
// @Tags Campaigns
// @Security SessionCookie
// @Accept json
// @Produce json
// @Success 201 {object} map[string]interface{}
// @Router /campaigns [post]
func swaggerCampaignsCreate() {}

// @Summary Get campaign
// @Tags Campaigns
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /campaigns/{id} [get]
func swaggerCampaignsGet() {}

// @Summary Patch campaign
// @Tags Campaigns
// @Security SessionCookie
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /campaigns/{id} [patch]
func swaggerCampaignsPatch() {}

// @Summary Delete campaign
// @Tags Campaigns
// @Security SessionCookie
// @Success 204
// @Router /campaigns/{id} [delete]
func swaggerCampaignsDelete() {}

// @Summary List campaign tasks
// @Tags Campaigns
// @Security SessionCookie
// @Produce json
// @Success 200 {array} map[string]interface{}
// @Router /campaigns/{id}/tasks [get]
func swaggerCampaignTasksList() {}

// @Summary Create campaign task
// @Tags Campaigns
// @Security SessionCookie
// @Accept json
// @Produce json
// @Success 201 {object} map[string]interface{}
// @Router /campaigns/{id}/tasks [post]
func swaggerCampaignTasksCreate() {}

// @Summary Get task
// @Tags Campaigns
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /tasks/{id} [get]
func swaggerTaskGet() {}

// @Summary Patch task
// @Tags Campaigns
// @Security SessionCookie
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /tasks/{id} [patch]
func swaggerTaskPatch() {}

// @Summary Delete task
// @Tags Campaigns
// @Security SessionCookie
// @Success 204
// @Router /tasks/{id} [delete]
func swaggerTaskDelete() {}

// @Summary Complete task
// @Tags Campaigns
// @Security SessionCookie
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /tasks/{id}/complete [post]
func swaggerTaskComplete() {}
