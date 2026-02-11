## ADDED Requirements

### Requirement: Text File Processing
The system SHALL read `.txt` files as-is without any cleaning or transformation.

#### Scenario: Plain text file
- **WHEN** a `.txt` file is detected for processing
- **THEN** the system reads its raw content and passes it directly to the chunker

### Requirement: DOCX File Cleaning
The system SHALL extract text content from `.docx` files and remove all non-text
elements including images, tables, formulas, headers, footers, and page numbers.

The system SHALL use `github.com/zakahan/docx2md` for docx-to-text conversion.

#### Scenario: DOCX with mixed content
- **WHEN** a `.docx` file containing text paragraphs, images, tables, and formulas is processed
- **THEN** only the text paragraphs are extracted; images, tables, formulas, headers, footers, and page numbers are discarded

#### Scenario: DOCX with only text
- **WHEN** a `.docx` file containing only text paragraphs is processed
- **THEN** all text content is preserved intact

### Requirement: Text Chunking
The system SHALL split cleaned text into chunks suitable for embedding.

Chunks SHALL be created by splitting on paragraph boundaries with a configurable
maximum chunk size (default 512 characters) and overlap (default 64 characters).

Each chunk SHALL retain metadata: source file path, chunk index, and character offset.

#### Scenario: Long document chunking
- **WHEN** a document with 5000 characters of text is processed with chunk_size=512 and overlap=64
- **THEN** the text is split into overlapping chunks, each no larger than 512 characters, with 64-character overlap between consecutive chunks

#### Scenario: Short document
- **WHEN** a document with 100 characters of text is processed
- **THEN** a single chunk is created containing the entire text
