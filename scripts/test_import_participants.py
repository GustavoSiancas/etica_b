"""Tests for CSV validation, identifier preservation and safe SQL generation."""
import csv
import io
from pathlib import Path
from uuid import uuid4
import unittest
from unittest.mock import patch

from scripts.import_participants import generate_sql, main, read_participants, sql_text


class ParticipantImportTests(unittest.TestCase):
    def setUp(self):
        test_root = Path(__file__).resolve().parents[1] / ".cache" / "import-tests"
        test_root.mkdir(parents=True, exist_ok=True)
        self.root = test_root / uuid4().hex
        self.root.mkdir()
        assert self.root.resolve().is_relative_to(test_root.resolve())
        self.addCleanup(self.cleanup_files)

    def cleanup_files(self):
        for path in self.root.iterdir():
            path.unlink()
        self.root.rmdir()

    def source(self, rows):
        path = self.root / "source.csv"
        with path.open("w", encoding="utf-8-sig", newline="") as f:
            writer = csv.writer(f, delimiter=";")
            writer.writerow(["numero_de_cal", "nombre_completo", "tipo_documento", "numero_documento"])
            writer.writerows(rows)
        return path

    def test_text_identifiers_unicode_and_sql_escaping(self):
        source = self.source([[21, "Luc\u00eda O'Neil\\Prueba", "dni", "00123456"], [22, "Another", "CE", "000123456"], [23, "Seven digits", "DNI", "8218687"]])
        result = read_participants(source)
        self.assertEqual(len(result.participants), 3)
        self.assertEqual(result.participants[0].document_number, "00123456")
        self.assertEqual(result.participants[1].document_number, "000123456")
        self.assertEqual(result.participants[2].document_number, "8218687")
        sql = generate_sql(result.participants)
        self.assertIn("Luc\u00eda O''Neil\\\\Prueba", sql)
        self.assertIn("WHERE d.registration_number = s.registration_number", sql)
        self.assertNotIn("DELETE FROM", sql)
        self.assertNotIn("UPDATE denunciados", sql)
        self.assertEqual(sql_text("a'\\b"), "E'a''\\\\b'")

    def test_invalid_rows_are_reported_without_overwriting_sql(self):
        source = self.source([[1, "Valid", "DNI", "00123456"], [2, "", "DNI", "12345678"], [3, "Invalid doc", "DNI", "NULL"], [4, "Invalid type", "RUC", "12345678"]])
        output = self.root / "seed.sql"
        report = self.root / "rejected.csv"
        output.write_text("previous SQL", encoding="utf-8")
        with patch("sys.stderr", new=io.StringIO()):
            self.assertEqual(main([str(source), "--output", str(output), "--report", str(report)]), 1)
        self.assertEqual(output.read_text(), "previous SQL")
        with report.open(encoding="utf-8", newline="") as f:
            self.assertEqual(len(list(csv.DictReader(f, delimiter=";"))), 3)
        with patch("sys.stdout", new=io.StringIO()):
            self.assertEqual(main([str(source), "--output", str(output), "--report", str(report), "--skip-invalid"]), 0)
        self.assertIn("E'Valid'", output.read_text())
        self.assertNotIn("Invalid doc", output.read_text())

    def test_cal_identity_is_distinct_from_document_identity(self):
        source = self.source([[1, "First", "DNI", "12345678"], [1, "Duplicate CAL", "CE", "000123456"], [2, "Different CAL", "DNI", "12345678"]])
        result = read_participants(source)
        self.assertEqual(len(result.participants), 2)
        self.assertEqual(len(result.rejected), 1)
        self.assertEqual(result.repeated_documents, 1)

    def test_missing_header_and_out_of_range_cal(self):
        source = self.root / "bad.csv"
        source.write_text("other;columns\n1;2\n")
        with self.assertRaises(ValueError):
            read_participants(source)
        result = read_participants(self.source([["9223372036854775808", "Out of range", "DNI", "12345678"]]))
        self.assertEqual(len(result.rejected), 1)
        with self.assertRaises(ValueError):
            generate_sql([])

    def test_input_cannot_be_overwritten_by_output_or_report(self):
        source = self.source([[1, "Valid", "DNI", "12345678"]])
        before = source.read_bytes()
        with patch("sys.stderr", new=io.StringIO()):
            self.assertEqual(main([str(source), "--output", str(source)]), 1)
        self.assertEqual(source.read_bytes(), before)


if __name__ == "__main__":
    unittest.main()
