package pagination

// GetPagination generates a slice of page numbers for pagination based on the current page, total pages, and the count of pages to display.
//
// Parameters:
// - page: The current page number.
// - totalPages: The total number of pages available.
// - count: The number of page numbers to display in the pagination.
//
// Returns:
// - A slice of uint32 containing the page numbers to be displayed in the pagination.
func GetPagination(page, totalPages, count uint64) []uint64 {
	var pagination []uint64

	// add page if first or last
	if page == 1 || page == totalPages {
		count++
	}

	if totalPages <= count {
		// If the total number of pages is less than or equal to the count, display all pages.
		for i := uint64(1); i <= totalPages; i++ {
			pagination = append(pagination, i)
		}
	} else {
		if page <= count/2 {
			// If the current page is in the first half of the pagination range, display the first 'count' pages.
			for i := uint64(1); i <= count; i++ {
				pagination = append(pagination, i)
			}
		} else if page >= totalPages-count/2 {
			// If the current page is in the last half of the pagination range, display the last 'count' pages.
			for i := totalPages - count + 1; i <= totalPages; i++ {
				pagination = append(pagination, i)
			}
		} else {
			// Otherwise, display 'count' pages centered around the current page.
			for i := page - count/2; i <= page+count/2; i++ {
				pagination = append(pagination, i)
			}
		}
	}

	// Add the first page if not in the slice
	if pagination[0] != 1 {
		pagination = append([]uint64{1}, pagination...)
	}

	// Add the last page if not in the slice
	if pagination[len(pagination)-1] != totalPages {
		pagination = append(pagination, totalPages)
	}

	return pagination
}
