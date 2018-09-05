package resource_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/Typeform/terraform-provider-kubectl/kubectl/resource"
)

var _ = Describe("ResourceHelper", func() {

	Describe("SplitYaml", func() {

		Context("When a single resource Yaml is passed", func() {

			var ()

			BeforeEach(func() {
			})

			It("Should parse the resource correctly when there is no leading newline", func() {

				const manifest = ("---\n" +
					"apiVersion: v1\n" +
					"kind: Namespace\n" +
					"metadata:\n" +
					"name: acceptance-test")

				resources := SplitYAMLDocument(manifest)
				Expect(len(resources)).To(Equal(1))
			})

			It("Should parse the resource correctly when there is no a leading newline", func() {

				const manifest = ("\n    \n" +
					"---\n" +
					"apiVersion: v1\n" +
					"kind: Namespace\n" +
					"metadata:\n" +
					"name: acceptance-test")

				resources := SplitYAMLDocument(manifest)
				Expect(len(resources)).To(Equal(1))
			})

			It("Should parse the resource correctly when there is no leading ---", func() {

				const manifest = ("---\n" +
					"apiVersion: v1\n" +
					"kind: Namespace\n" +
					"metadata:\n" +
					"name: acceptance-test")

				resources := SplitYAMLDocument(manifest)
				Expect(len(resources)).To(Equal(1))
			})

			It("Should parse the resource correctly when there is no leading ---", func() {

				const manifest = ("apiVersion: v1\n" +
					"kind: Namespace\n" +
					"metadata:\n" +
					"name: acceptance-test")

				resources := SplitYAMLDocument(manifest)
				Expect(len(resources)).To(Equal(1))
			})
		})

		Context("When a multiple resources Yaml is passed", func() {

			var ()

			BeforeEach(func() {

			})

			It("Should parse multiple resources correctly", func() {

				const manifest = ("---\n" +
					"apiVersion: v1\n" +
					"kind: Namespace\n" +
					"metadata:\n" +
					"name: acceptance-test\n" +
					"---\n" +
					"apiVersion: v1\n" +
					"kind: Namespace\n" +
					"metadata:\n" +
					"name: acceptance-test")

				resources := SplitYAMLDocument(manifest)
				Expect(len(resources)).To(Equal(2))
			})

		})

	})

})
