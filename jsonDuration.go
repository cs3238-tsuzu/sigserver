package main

import "time"

type jsonDuration struct {
	time.Duration
}

func (j *jsonDuration) MarshalJSON() ([]byte, error) {
	return []byte(j.String()), nil
}

func (j *jsonDuration) MarshalTOML() ([]byte, error) {
	return []byte(j.String()), nil
}

func (j *jsonDuration) UnmarshalJSON(data []byte) error {
	d, err := time.ParseDuration(string(data))

	if err != nil {
		return err
	}

	j.Duration = d

	return nil
}
func (j *jsonDuration) UnmarshalTOML(data []byte) error {
	d, err := time.ParseDuration(string(data))

	if err != nil {
		return err
	}

	j.Duration = d

	return nil
}
