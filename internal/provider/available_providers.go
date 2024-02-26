package provider

type ProviderType int

const (
	BatoId  ProviderType = iota
	AsuraId ProviderType = iota
)

func GetProviderByType(typeId ProviderType) Provider {
	switch typeId {
	case BatoId:
		return &Bato{}
	case AsuraId:
		return &Asura{}
	}
	return nil
}
