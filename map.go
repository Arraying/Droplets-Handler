package main

// get gets a droplet by identifier.
func (d *dropletMap) get(identifier string) *droplet {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	return d.droplets[identifier]
}

// put adds a new droplet to the map.
func (d *dropletMap) put(identifier string, droplet *droplet) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.droplets[identifier] = droplet
}

// contains checks if the map contains the droplet.
func (d *dropletMap) contains(identifier string) bool {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	_, contains := d.droplets[identifier]
	return contains
}

// remove removes a droplet.
func (d *dropletMap) remove(identifier string) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	delete(d.droplets, identifier)
}

func (d *dropletMap) forAllDroplets(fn func(*droplet)) {
	for _, droplet := range d.droplets {
		fn(droplet)
	}
}
