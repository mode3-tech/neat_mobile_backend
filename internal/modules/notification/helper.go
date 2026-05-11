package notification

func normalizeNotificationPagination(page, pageSize int) (int, int, int) {
	if page < 1 {
		page = defaultNotificationPage
	}

	if pageSize < 1 {
		pageSize = defaultNotificationPageSize
	}

	if pageSize > maxNotificationPageSize {
		pageSize = maxNotificationPageSize
	}

	offset := (page - 1) * pageSize

	return page, pageSize, offset
}
