Personal document library and import artifacts.

Organize source documents by topic for bundling:

  svalbard import --bundle radio-manuals library/radio/*.pdf
  svalbard import --bundle medical-refs library/medical/*

Import artifacts (ZIMs from web crawls, media, bundles) also
land here alongside .source.yaml provenance metadata.

Attach any registered source to a drive with:

  svalbard attach local:radio-manuals /path/to/drive
